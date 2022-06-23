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
	"encoding/base64"
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

	clusterv1alpha1 "github.com/redhat-et/ipfs-operator/api/v1alpha1"
)

const (
	finalizer = "openshift.ifps.cluster"
)

// IpfsReconciler reconciles a Ipfs object
type IpfsReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=*,resources=*,verbs=get;list
//+kubebuilder:rbac:groups=apps,resources=statefulsets,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=cluster.ipfs.io,resources=ipfs,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=cluster.ipfs.io,resources=ipfs/finalizers,verbs=update
//+kubebuilder:rbac:groups=cluster.ipfs.io,resources=ipfs/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=core,resources=configmaps,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=core,resources=persistentvolumeclaims,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=core,resources=secrets,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=core,resources=serviceaccounts,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=core,resources=services,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=networking.k8s.io,resources=ingresses,verbs=get;list;watch;create;update;patch;delete

func (r *IpfsReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := ctrllog.FromContext(ctx)
	// Fetch the Ipfs instance
	instance := &clusterv1alpha1.Ipfs{}
	if err := r.Get(ctx, req.NamespacedName, instance); err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			log.Info("Ipfs resource not found. Ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		// Error reading the object - requeue the request.
		log.Error(err, "Failed to get Ipfs")
		return ctrl.Result{}, err
	}

	// Add finalizer for this CR
	if !controllerutil.ContainsFinalizer(instance, finalizer) {
		controllerutil.AddFinalizer(instance, finalizer)
		err := r.Update(ctx, instance)
		if err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{Requeue: true}, nil
	}

	if instance.DeletionTimestamp != nil {
		controllerutil.RemoveFinalizer(instance, finalizer)
		return ctrl.Result{}, r.Update(ctx, instance)
	}

	priv, peerid, err := newKey()
	if err != nil {
		log.Error(err, "cannot generate new key")
		return ctrl.Result{}, err
	}
	privBytes, err := priv.Bytes()
	if err != nil {
		log.Error(err, "cannot get bytes from private key")
		return ctrl.Result{}, err
	}
	privStr := base64.StdEncoding.EncodeToString(privBytes)

	clusSec, err := newClusterSecret()
	if err != nil {
		log.Error(err, "cannot generate new cluster secret")
		return ctrl.Result{}, err
	}

	// Create circuit relays, if necessary.
	// TODO: if we change the number of CircuitRelays, we should update
	// the IPFS config file as well.
	if len(instance.Status.CircuitRelays) < int(instance.Spec.CircuitRelays) {
		var relayNames []string
		for i := 0; int32(i) < instance.Spec.CircuitRelays; i++ {
			name := fmt.Sprintf("%s-%d", instance.Name, i)
			relayNames = append(relayNames, name)
			relay := clusterv1alpha1.CircuitRelay{}
			relay.Name = name
			relay.Namespace = instance.Namespace
			ctrl.SetControllerReference(instance, &relay, r.Scheme)
			r.Create(ctx, &relay)
			instance.Status.CircuitRelays = append(instance.Status.CircuitRelays, relay.Name)
		}
		r.Status().Update(ctx, instance)
	}

	// Check the status of circuit relays.
	// wait for them to complte so we can determine announce addresses.
	if len(instance.Status.Addresses) == 0 {
		for _, relayName := range instance.Status.CircuitRelays {
			relay := clusterv1alpha1.CircuitRelay{}
			relay.Name = relayName
			relay.Namespace = instance.Namespace
			err := r.Get(ctx, client.ObjectKeyFromObject(&relay), &relay)
			if err != nil {
				log.Error(err, "could not lookup circuitRelay", "relay", relayName)
				return ctrl.Result{Requeue: true}, err
			}
			if len(relay.Status.AnnounceAddrs) == 0 || relay.Status.PeerID == "" {
				log.Info("relay is not ready yet. Will continue waiting.", "relay", relayName)
				return ctrl.Result{RequeueAfter: time.Minute}, nil
			}
			for _, addr := range relay.Status.AnnounceAddrs {
				instance.Status.Addresses = append(instance.Status.Addresses, fmt.Sprintf("%s/p2p/%s", addr, relay.Status.PeerID))
			}
		}
		r.Status().Update(ctx, instance)
	}

	sa := corev1.ServiceAccount{}
	svc := corev1.Service{}
	cmScripts := corev1.ConfigMap{}
	cmConfig := corev1.ConfigMap{}
	secConfig := corev1.Secret{}
	sts := appsv1.StatefulSet{}

	mutsa := r.serviceAccount(instance, &sa)
	mutsvc, svcName := r.serviceCluster(instance, &svc)
	mutCmScripts, cmScriptName := r.configMapScripts(instance, &cmScripts)
	mutCmConfig, cmConfigName := r.configMapConfig(instance, &cmConfig, peerid.String())
	mutSecConfig, secConfigName := r.secretConfig(instance, &secConfig, []byte(clusSec), []byte(privStr))
	mutSts := r.statefulSet(instance, &sts, svcName, secConfigName, cmConfigName, cmScriptName)

	trackedObjects := map[client.Object]controllerutil.MutateFn{
		&sa:        mutsa,
		&svc:       mutsvc,
		&cmScripts: mutCmScripts,
		&cmConfig:  mutCmConfig,
		&secConfig: mutSecConfig,
		&sts:       mutSts,
	}

	var requeue bool
	for obj, mut := range trackedObjects {
		kind := obj.GetObjectKind().GroupVersionKind()
		name := obj.GetName()
		result, err := controllerutil.CreateOrPatch(ctx, r.Client, obj, mut)
		if err != nil {
			log.Error(err, "error creating object", "objname", name, "objKind", kind.Kind, "result", result)
			requeue = true
		} else {
			log.Info("object changed", "objName", name, "objKind", kind.Kind, "result", result)
		}
	}
	return ctrl.Result{Requeue: requeue}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *IpfsReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&clusterv1alpha1.Ipfs{}).
		Owns(&appsv1.StatefulSet{}, builder.OnlyMetadata).
		Owns(&corev1.Service{}, builder.OnlyMetadata).
		Owns(&corev1.ServiceAccount{}, builder.OnlyMetadata).
		Owns(&corev1.Secret{}, builder.OnlyMetadata).
		Owns(&corev1.ConfigMap{}, builder.OnlyMetadata).
		Owns(&clusterv1alpha1.Ipfs{}, builder.OnlyMetadata).
		WithOptions(controller.Options{
			MaxConcurrentReconciles: 1,
		}).Complete(r)
}
