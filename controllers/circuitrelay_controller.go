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

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/json"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	crypto "github.com/libp2p/go-libp2p-crypto"
	clusterv1alpha1 "github.com/redhat-et/ipfs-operator/api/v1alpha1"

	ma "github.com/multiformats/go-multiaddr"
)

// CircuitRelayReconciler reconciles a CircuitRelay object
type CircuitRelayReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=*,resources=*,verbs=get;list
//+kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
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
		var iptype, address string
		if ip.To4() != nil {
			iptype = "ip4"
			address = addr.IP
		} else if ip.To16() != nil {
			iptype = "ip6"
			address = addr.IP
		} else {
			iptype = "dns4"
			address = addr.Hostname
		}
		for j, port := range svc.Spec.Ports {
			p := strings.ToLower(string(port.Protocol))
			mastr := fmt.Sprintf("/%s/%s/%s/%d", iptype, address, p, port.Port)
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

	trackedObjects := make(map[client.Object]controllerutil.MutateFn)

	// Test if we have already updated the status.
	// And if not, then generate a new identity
	if instance.Status.AddrInfo.ID == "" {

		addrStrings := make([]string, len(maddrs))
		for i, addr := range maddrs {
			addrStrings[i] = addr.String()
		}
		instance.Status.AddrInfo.Addrs = addrStrings
		privkey, pubkey, err := newKey()
		if err != nil {
			log.Error(err, "error during key generation")
			return ctrl.Result{
				RequeueAfter: time.Minute,
			}, err
		}
		instance.Status.AddrInfo.ID = pubkey.String()
		r.Status().Update(ctx, instance)

		identity, err := crypto.MarshalPrivateKey(privkey)
		if err != nil {
			log.Error(err, "error marshaling private key")
			return ctrl.Result{
				RequeueAfter: time.Minute,
			}, err
		}
		sec := corev1.Secret{}
		mutsec := r.secretIdentity(instance, &sec, identity)
		trackedObjects[&sec] = mutsec
	}

	if err := instance.Status.AddrInfo.Parse(); err != nil {
		log.Error(err, "cannot parse AddrInfo for relay.")
		return ctrl.Result{
			Requeue: true,
		}, err
	}

	log.Info("create or patch circuitrelay deployment with addrs", "addrs", maddrs)

	cm := corev1.ConfigMap{}
	mutcm := r.configRelay(instance, &cm)
	trackedObjects[&cm] = mutcm

	dep := appsv1.Deployment{}
	mutdep := r.deploymentRelay(instance, &dep)
	trackedObjects[&dep] = mutdep

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
func (r *CircuitRelayReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		// Uncomment the following line adding a pointer to an instance of the controlled resource as an argument
		For(&clusterv1alpha1.CircuitRelay{}).
		Owns(&corev1.Service{}, builder.OnlyMetadata).
		Owns(&appsv1.Deployment{}, builder.OnlyMetadata).
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
				// Commented out because some load balancers don't support TCP+UDP:(
				//
				// {
				// 	Name:       "swarm-udp",
				// 	Protocol:   corev1.ProtocolUDP,
				// 	Port:       4001,
				// 	TargetPort: intstr.FromString("swarm-udp"),
				// },
				//
				// Commented out.
				// TODO: support websockets and TLS.
				// {
				// 	Name:       "swarm-ws",
				// 	Protocol:   corev1.ProtocolTCP,
				// 	Port:       8081,
				// 	TargetPort: intstr.FromString("swarm-ws"),
				// },
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

func (r *CircuitRelayReconciler) secretIdentity(m *clusterv1alpha1.CircuitRelay, sec *corev1.Secret, identity []byte) controllerutil.MutateFn {
	secName := "libp2p-relay-daemon-identity-" + m.Name
	expected := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secName,
			Namespace: m.Namespace,
		},
		Data: map[string][]byte{
			"identity": identity,
		},
	}
	expected.DeepCopyInto(sec)
	ctrl.SetControllerReference(m, sec, r.Scheme)
	return func() error {
		return nil
	}
}

func (r *CircuitRelayReconciler) configRelay(m *clusterv1alpha1.CircuitRelay, cm *corev1.ConfigMap) controllerutil.MutateFn {
	cmName := "libp2p-relay-daemon-config-" + m.Name
	announceAddrs := m.Status.AddrInfo.Addrs
	cfg := map[string]interface{}{
		"Network": map[string]interface{}{
			"AnnounceAddrs": announceAddrs,
		},
	}
	cfgbytes, _ := json.Marshal(cfg)
	expected := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cmName,
			Namespace: m.Namespace,
		},
		BinaryData: map[string][]byte{
			"config.json": cfgbytes,
		},
	}
	expected.DeepCopyInto(cm)
	ctrl.SetControllerReference(m, cm, r.Scheme)
	return func() error {
		return nil
	}
}

func (r *CircuitRelayReconciler) deploymentRelay(m *clusterv1alpha1.CircuitRelay, dep *appsv1.Deployment) controllerutil.MutateFn {
	depName := "libp2p-relay-daemon-" + m.Name
	expected := appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      depName,
			Namespace: m.Namespace,
		},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app.kubernetes.io/name": depName,
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app.kubernetes.io/name": depName,
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "relay",
							Image: "coryschwartz/libp2p-relay-daemon:latest",
							Args: []string{
								"-config",
								"/config.json",
								"-id",
								"/identity",
							},
							Ports: []corev1.ContainerPort{
								{
									Name:          "swarm",
									ContainerPort: 4001,
									Protocol:      "TCP",
								},
								{
									Name:          "swarm-udp",
									ContainerPort: 4001,
									Protocol:      "UDP",
								},
								{
									Name:          "pprof",
									ContainerPort: 6060,
									Protocol:      "UDP",
								},
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "config",
									MountPath: "/config.json",
									SubPath:   "config.json",
								},
								{
									Name:      "identity",
									MountPath: "/identity",
									SubPath:   "identity",
								},
							},
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: "config",
							VolumeSource: corev1.VolumeSource{
								ConfigMap: &corev1.ConfigMapVolumeSource{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: "libp2p-relay-daemon-config-" + m.Name,
									},
								},
							},
						},
						{
							Name: "identity",
							VolumeSource: corev1.VolumeSource{
								Secret: &corev1.SecretVolumeSource{
									SecretName: "libp2p-relay-daemon-identity-" + m.Name,
								},
							},
						},
					},
				},
			},
		},
	}
	expected.DeepCopyInto(dep)
	ctrl.SetControllerReference(m, dep, r.Scheme)
	return func() error {
		dep.Spec = expected.Spec
		return nil
	}
}
