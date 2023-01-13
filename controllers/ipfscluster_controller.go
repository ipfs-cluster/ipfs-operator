/*
Copyright 2021.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controllers

import (
	"context"
	"fmt"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/libp2p/go-libp2p-core/peer"
	clusterv1alpha1 "github.com/redhat-et/ipfs-operator/api/v1alpha1"
	"github.com/redhat-et/ipfs-operator/controllers/utils"
)

const (
	finalizer = "openshift.ifps.cluster"
)

// IpfsClusterReconciler reconciles a Ipfs object.
type IpfsClusterReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=*,resources=*,verbs=get;list
//+kubebuilder:rbac:groups=apps,resources=statefulsets,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=cluster.ipfs.io,resources=ipfsclusters,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=cluster.ipfs.io,resources=ipfsclusters/finalizers,verbs=update
//+kubebuilder:rbac:groups=cluster.ipfs.io,resources=ipfsclusters/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=core,resources=configmaps,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=core,resources=persistentvolumeclaims,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=core,resources=secrets,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=core,resources=serviceaccounts,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=core,resources=services,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=networking.k8s.io,resources=ingresses,verbs=get;list;watch;create;update;patch;delete

func (r *IpfsClusterReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := ctrllog.FromContext(ctx)
	// Fetch the Ipfs instance
	instance, err := r.ensureIPFSCluster(ctx, req)
	if err != nil {
		return ctrl.Result{}, err
	}

	// Add finalizer for this CR
	if !controllerutil.ContainsFinalizer(instance, finalizer) {
		controllerutil.AddFinalizer(instance, finalizer)
		err = r.Update(ctx, instance)
		if err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{Requeue: true}, nil
	}

	if instance.DeletionTimestamp != nil {
		controllerutil.RemoveFinalizer(instance, finalizer)
		return ctrl.Result{}, r.Update(ctx, instance)
	}

	// generate a new ID
	var peerid peer.ID
	var privStr string
	if peerid, privStr, err = generateIdentity(); err != nil {
		log.Error(err, "failed to generate identity")
		return ctrl.Result{}, err
	}

	clusSec, err := newClusterSecret()
	if err != nil {
		log.Error(err, "cannot generate new cluster secret")
		return ctrl.Result{}, err
	}

	if err = r.createCircuitRelays(ctx, instance); err != nil {
		log.Error(err, "cannot create circuit relays")
		return ctrl.Result{}, err
	}

	// Check the status of circuit relays.
	// wait for them to complte so we can determine announce addresses.
	for _, relayName := range instance.Status.CircuitRelays {
		relay := clusterv1alpha1.CircuitRelay{}
		relay.Name = relayName
		relay.Namespace = instance.Namespace
		if err = r.Client.Get(ctx, client.ObjectKeyFromObject(&relay), &relay); err != nil {
			log.Error(err, "could not lookup circuitRelay", "relay", relayName)
			return ctrl.Result{Requeue: true}, err
		}
		if relay.Status.AddrInfo.ID == "" {
			log.Info("relay is not ready yet. Will continue waiting.", "relay", relayName)
			return ctrl.Result{RequeueAfter: time.Second * 15}, nil
		}
	}
	if err = r.Status().Update(ctx, instance); err != nil {
		return ctrl.Result{}, err
	}

	// Reconcile the tracked objects
	trackedObjects := r.createTrackedObjects(ctx, instance, peerid, clusSec, privStr)
	shouldRequeue := utils.CreateOrPatchTrackedObjects(ctx, trackedObjects, r.Client, log)
	return ctrl.Result{Requeue: shouldRequeue}, nil
}

// createTrackedObjects Creates a mapping from client objects to their mutating functions.
func (r *IpfsClusterReconciler) createTrackedObjects(
	ctx context.Context,
	instance *clusterv1alpha1.IpfsCluster,
	peerID peer.ID,
	clusterSecret string,
	privateString string,
) map[client.Object]controllerutil.MutateFn {
	sa := corev1.ServiceAccount{}
	svc := corev1.Service{}
	gwsvc := corev1.Service{}
	apisvc := corev1.Service{}
	cmScripts := corev1.ConfigMap{}
	secConfig := corev1.Secret{}
	sts := appsv1.StatefulSet{}

	mutsa := r.serviceAccount(instance, &sa)
	mutsvc, svcName := r.serviceCluster(instance, &svc)

	mutCmScripts, cmScriptName := r.ConfigMapScripts(ctx, instance, &cmScripts)
	mutSecConfig, secConfigName := r.SecretConfig(
		ctx,
		instance,
		&secConfig,
		[]byte(clusterSecret),
		[]byte(peerID.String()),
		[]byte(privateString),
	)
	mutSts := r.StatefulSet(instance, &sts, svcName, secConfigName, cmScriptName)

	trackedObjects := map[client.Object]controllerutil.MutateFn{
		&sa:        mutsa,
		&svc:       mutsvc,
		&cmScripts: mutCmScripts,
		&secConfig: mutSecConfig,
		&sts:       mutSts,
	}

	if instance.Spec.Gateway != nil {
		mutgwsvc, _ := r.serviceGateway(instance, &gwsvc)
		if instance.Spec.Gateway.Strategy == clusterv1alpha1.ExternalStrategyLoadBalancer {
			trackedObjects[&gwsvc] = mutgwsvc
		}
	}
	if instance.Spec.ClusterAPI != nil {
		mutapisvc, _ := r.serviceAPI(instance, &apisvc)
		if instance.Spec.ClusterAPI.Strategy == clusterv1alpha1.ExternalStrategyLoadBalancer {
			trackedObjects[&apisvc] = mutapisvc
		}
	}
	return trackedObjects
}

// ensureIPFSCluster Attempts to obtain an IPFS Cluster resource, and error if not found.
func (r *IpfsClusterReconciler) ensureIPFSCluster(
	ctx context.Context,
	req ctrl.Request,
) (*clusterv1alpha1.IpfsCluster, error) {
	var err error
	instance := &clusterv1alpha1.IpfsCluster{}
	if err = r.Get(ctx, req.NamespacedName, instance); err == nil {
		return instance, nil
	}
	if errors.IsNotFound(err) {
		// Request object not found, could have been deleted after reconcile request.
		// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
		// Return and don't requeue
		return nil, fmt.Errorf("IPFS Cluster resource not found, ignoring since object must be deleted: %w", err)
	}
	// Error reading the object - requeue the request.
	return nil, fmt.Errorf("failed to get Ipfs: %w", err)
}

// createCircuitRelays Creates the necessary amount of circuit relays if any are missing.
// FIXME: if we change the number of CircuitRelays, we should update
// the IPFS config file as well.
func (r *IpfsClusterReconciler) createCircuitRelays(
	ctx context.Context,
	instance *clusterv1alpha1.IpfsCluster,
) error {
	// do nothing
	if len(instance.Status.CircuitRelays) >= int(instance.Spec.Networking.CircuitRelays) {
		// FIXME: handle scale-down of circuit relays
		return nil
	}
	// create the CircuitRelays
	for i := 0; int32(i) < instance.Spec.Networking.CircuitRelays; i++ {
		name := fmt.Sprintf("%s-%d", instance.Name, i)
		relay := clusterv1alpha1.CircuitRelay{}
		relay.Name = name
		relay.Namespace = instance.Namespace
		if err := ctrl.SetControllerReference(instance, &relay, r.Scheme); err != nil {
			return fmt.Errorf(
				"cannot set controller reference for new circuitRelay: %w, circuitRelay: %s",
				err, relay.Name,
			)
		}
		if err := r.Create(ctx, &relay); err != nil {
			return fmt.Errorf("cannot create new circuitRelay: %w", err)
		}
		instance.Status.CircuitRelays = append(instance.Status.CircuitRelays, relay.Name)
	}
	if err := r.Status().Update(ctx, instance); err != nil {
		return err
	}
	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *IpfsClusterReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&clusterv1alpha1.IpfsCluster{}).
		Owns(&appsv1.StatefulSet{}, builder.OnlyMetadata).
		Owns(&corev1.Service{}, builder.OnlyMetadata).
		Owns(&corev1.ServiceAccount{}, builder.OnlyMetadata).
		Owns(&corev1.Secret{}, builder.OnlyMetadata).
		Owns(&corev1.ConfigMap{}, builder.OnlyMetadata).
		Owns(&clusterv1alpha1.IpfsCluster{}, builder.OnlyMetadata).
		WithOptions(controller.Options{
			MaxConcurrentReconciles: 1,
		}).Complete(r)
}
