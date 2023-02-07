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

	"github.com/libp2p/go-libp2p/core/peer"
	ma "github.com/multiformats/go-multiaddr"
	"github.com/redhat-et/ipfs-operator/api/v1alpha1"
	clusterv1alpha1 "github.com/redhat-et/ipfs-operator/api/v1alpha1"
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
	failResult := ctrl.Result{RequeueAfter: time.Second * 15}
	// Fetch the Ipfs instance
	instance, err := r.ensureIPFSCluster(ctx, req)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to ensure ipfscluster: %w", err)
	}

	// Add finalizer for this CR
	if !controllerutil.ContainsFinalizer(instance, finalizer) {
		controllerutil.AddFinalizer(instance, finalizer)
		err = r.Update(ctx, instance)
		if err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to update instance: %w", err)
		}
		return ctrl.Result{Requeue: true}, nil
	}

	if instance.DeletionTimestamp != nil {
		controllerutil.RemoveFinalizer(instance, finalizer)
		return ctrl.Result{}, r.Update(ctx, instance)
	}

	// Reconcile the tracked objects
	err = r.createTrackedObjects(ctx, instance)
	if err != nil {
		log.Error(err, "failed to reconcile ipfscluster")
		return failResult, err
	}
	return ctrl.Result{Requeue: false}, nil
}

// createTrackedObjects Creates a mapping from client objects to their mutating functions.
func (r *IpfsClusterReconciler) createTrackedObjects(
	ctx context.Context,
	instance *clusterv1alpha1.IpfsCluster,
) error {
	var err error
	var svc *corev1.Service
	var secret *corev1.Secret
	var cmScripts *corev1.ConfigMap
	var relayPeers []peer.AddrInfo
	var relayStatic []ma.Multiaddr
	var bootstrapPeers []string

	if _, err = r.ensureSA(ctx, instance); err != nil {
		return fmt.Errorf("retrieved error while ensuring SA: %w", err)
	}
	if svc, err = r.ensureServiceCluster(ctx, instance); err != nil {
		return fmt.Errorf("could not ensure service cluster: %w", err)
	}
	if secret, err = r.EnsureSecretConfig(ctx, instance); err != nil {
		return fmt.Errorf("failed to ensure secret config: %w", err)
	}
	if err = r.EnsureCircuitRelay(ctx, instance, secret); err != nil {
		return fmt.Errorf("failed to ensure circuit relays: %w", err)
	}
	if relayPeers, relayStatic, err = r.EnsureRelayCircuitInfo(ctx, instance); err != nil {
		return fmt.Errorf("could not retrieve information from the relay circuit: %w", err)
	}
	// initialize bootstrap peers if we are on a private network
	if instance.Spec.Networking.NetworkMode == v1alpha1.NetworkModePrivate {
		if instance.Spec.Replicas < 1 {
			return fmt.Errorf("number of replicas must be at least 1 to run in private mode")
		}
		if bootstrapPeers, err = getBootstrapAddrs(secret, relayPeers); err != nil {
			return fmt.Errorf("could not configure private network: %w", err)
		}
	}
	if cmScripts, err = r.EnsureConfigMapScripts(ctx, instance, relayPeers, relayStatic, bootstrapPeers); err != nil {
		return fmt.Errorf("could not ensure configmap scripts: %w", err)
	}
	if _, err = r.StatefulSet(ctx, instance, svc.Name, secret.ObjectMeta.Name, cmScripts.ObjectMeta.Name); err != nil {
		return fmt.Errorf("could not ensure statefulset: %w", err)
	}
	return nil
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

func getBootstrapAddrs(secret *corev1.Secret, relayPeers []peer.AddrInfo) ([]string, error) {
	bootstrapPeers := []string{}
	peer0IDKey := KeyPeerIDPrefix + "0"
	peer0ID, ok := secret.Data[peer0IDKey]
	if !ok {
		return nil, fmt.Errorf("could not retrieve initial peer id")
	}
	initPeerID := string(peer0ID)
	// construct a p2p circuit here
	for _, peer := range relayPeers {
		circuitID := peer.ID
		for _, circuitAddr := range peer.Addrs {
			bootstrapAddr := fmt.Sprintf("%s/p2p/%s/p2p-circuit/p2p/%s", circuitAddr.String(), circuitID.String(), initPeerID)
			bootstrapPeers = append(bootstrapPeers, bootstrapAddr)
		}
	}
	return bootstrapPeers, nil
}
