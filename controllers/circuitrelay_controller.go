/*
Copyright 2022.

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
	"net"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	clusterv1alpha1 "github.com/redhat-et/ipfs-operator/api/v1alpha1"

	ma "github.com/multiformats/go-multiaddr"
)

// CircuitRelayReconciler reconciles a CircuitRelay object
type CircuitRelayReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=cluster.ipfs.io,resources=circuitrelays,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=cluster.ipfs.io,resources=circuitrelays/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=cluster.ipfs.io,resources=circuitrelays/finalizers,verbs=update

func (r *CircuitRelayReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	instance := &clusterv1alpha1.CircuitRelay{}
	if err := r.Get(ctx, req.NamespacedName, instance); err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			log.Info("CircuitRelay  resource not found. Ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		// Error reading the object - requeue the request.
		log.Error(err, "Failed to get CircuitRelay")
		return ctrl.Result{}, err
	}

	svc := corev1.Service{}
	svcMut := r.serviceRelay(instance, &svc)
	_, err := controllerutil.CreateOrPatch(ctx, r.Client, &svc, svcMut)
	if err != nil {
		log.Error(err, "error during CreateOrPatch service", "name", instance.Name)
		return ctrl.Result{
			Requeue: true,
		}, err
	}

	// Check that we have a public address
	// if not, wait until it's available.
	rsvc := corev1.Service{}
	err = r.Get(ctx, client.ObjectKeyFromObject(&svc), &rsvc)
	if err != nil {
		log.Error(err, "error looking up service", "name", instance.Name)
		return ctrl.Result{
			Requeue: true,
		}, err
	}

	if len(rsvc.Status.LoadBalancer.Ingress) == 0 {
		log.Info("still waiting for service ingress addresses", "name", instance.Name)
		return ctrl.Result{
			RequeueAfter: time.Minute,
		}, err
	}

	maddrs := make([]ma.Multiaddr, len(rsvc.Status.LoadBalancer.Ingress)*len(rsvc.Spec.Ports))
	for i, addr := range rsvc.Status.LoadBalancer.Ingress {
		ip := net.ParseIP(addr.IP)
		var iptype string
		if ip.To4() == nil {
			iptype = "ip6"
		} else {
			iptype = "ip4"
		}
		for j, port := range svc.Spec.Ports {
			p := strings.ToLower(string(port.Protocol))
			mastr := fmt.Sprintf("/%s/%s/%s/%d", iptype, addr.IP, p, port.Port)
			maddr, err := ma.NewMultiaddr(mastr)
			if err != nil {
				log.Error(err, "cannot parse multiaddr", "ip", addr.IP)
				return ctrl.Result{
					Requeue: true,
				}, err
			}
			idx := (i * len(svc.Spec.Ports)) + j
			maddrs[idx] = maddr
		}
	}

	mastrings := make([]string, len(maddrs))
	for i, maddr := range maddrs {
		mastrings[i] = maddr.String()
	}
	instance.Status.AnnounceAddrs = mastrings

	r.Status().Update(ctx, instance)

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *CircuitRelayReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		// Uncomment the following line adding a pointer to an instance of the controlled resource as an argument
		For(&clusterv1alpha1.CircuitRelay{}).
		Owns(&corev1.Service{}, builder.OnlyMetadata).
		Complete(r)
}

func (r *CircuitRelayReconciler) serviceRelay(m *clusterv1alpha1.CircuitRelay, svc *corev1.Service) controllerutil.MutateFn {
	svcName := "libp2p-relay-daemon-" + m.Name
	expected := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      svcName,
			Namespace: m.Namespace,
		},
		Spec: corev1.ServiceSpec{
			Type: corev1.ServiceTypeLoadBalancer,
			Ports: []corev1.ServicePort{
				{
					Name:       "swarm",
					Protocol:   corev1.ProtocolTCP,
					Port:       4001,
					TargetPort: intstr.FromString("swarm"),
				},
				// Commented out because some load balancers don't support UDP+HTTP :(
				//
				// {
				// 	Name:       "swarm-udp",
				// 	Protocol:   corev1.ProtocolUDP,
				// 	Port:       4001,
				// 	TargetPort: intstr.FromString("swarm-udp"),
				// },
				{
					Name:       "swarm-ws",
					Protocol:   corev1.ProtocolTCP,
					Port:       8081,
					TargetPort: intstr.FromString("swarm-ws"),
				},
			},
			Selector: map[string]string{
				"app.kubernetes.io/name": svcName,
			},
		},
	}
	expected.DeepCopyInto(svc)
	ctrl.SetControllerReference(m, svc, r.Scheme)
	return func() error {
		svc.Spec = expected.Spec
		return nil
	}
}
