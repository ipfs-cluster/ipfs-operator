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
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
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

// IpfsReconciler reconciles a Export object
type IpfsReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=cluster.ipfs.io,resources=ipfs,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=cluster.ipfs.io,resources=ipfs/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=cluster.ipfs.io,resources=ipfs/finalizers,verbs=update
//+kubebuilder:rbac:groups=core,resources=serviceaccounts,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=core,resources=services,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=apps,resources=statefulsets,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=core,resources=persistentvolumeclaims,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=networking.k8s.io,resources=ingresses,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=*,resources=*,verbs=get;list

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the Export object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.8.3/pkg/reconcile

func (r *IpfsReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := ctrllog.FromContext(ctx)
	// Fetch the Export instance
	instance := &clusterv1alpha1.Ipfs{}
	if err := r.Get(ctx, req.NamespacedName, instance); err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			log.Info("Export resource not found. Ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		// Error reading the object - requeue the request.
		log.Error(err, "Failed to get Export")
		return ctrl.Result{}, err
	}

	// Add finalizer for this CR
	if !controllerutil.ContainsFinalizer(instance, finalizer) {
		controllerutil.AddFinalizer(instance, finalizer)
		err := r.Update(ctx, instance)
		if err != nil {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{Requeue: true}, nil
	}

	if instance.DeletionTimestamp != nil {
		// clean up
		err := r.CleanUpOpjects(ctx, instance)
		if err != nil {
			return ctrl.Result{}, err
		}

		controllerutil.RemoveFinalizer(instance, finalizer)
		err = r.Update(ctx, instance)
		if err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

	// Check if statefulset already exists, if not create a new one
	if instance.Spec.Public && instance.Status.Address != "" || !instance.Spec.Public {
		foundiSS := &appsv1.StatefulSet{}
		if err := r.Get(ctx, types.NamespacedName{Name: "ipfs-" + instance.Name, Namespace: instance.Namespace}, foundiSS); err != nil {
			if errors.IsNotFound(err) {
				// Define a new statefulset
				statefulSet := r.iSSGenerate(instance)
				log.Info("Creating a new StatefulSet", "StatefulSet.Namespace", statefulSet.Namespace, "StatefulSet.Name", statefulSet.Name)
				if err := r.Create(ctx, statefulSet); err != nil {
					log.Error(err, "Failed to create new StatefulSet", "statefulSet.Namespace", statefulSet.Namespace, "statefulSet.Name", statefulSet.Name)

					return ctrl.Result{}, err
				}
				if err := wait.Poll(time.Second*1, time.Second*15, func() (done bool, err error) {
					if err := r.Get(ctx, types.NamespacedName{Name: "ipfs-" + instance.Name, Namespace: instance.Namespace}, foundiSS); err != nil {
						if errors.IsNotFound(err) {
							return false, nil
						} else {
							return false, err
						}
					}
					return true, nil
				}); err != nil {
					return ctrl.Result{}, err
				}
				// StatefulSet created successfully - return and requeue
				return ctrl.Result{Requeue: true}, nil
			}
			log.Error(err, "Failed to get StatefulSet")
		}
	}

	// Check if statefulset already exists, if not create a new one
	foundcSS := &appsv1.StatefulSet{}
	if err := r.Get(ctx, types.NamespacedName{Name: "cluster-" + instance.Name, Namespace: instance.Namespace}, foundcSS); err != nil {
		if errors.IsNotFound(err) {
			// Define a new statefulset
			statefulSet := r.cSSGenerate(instance)
			log.Info("Creating a new StatefulSet", "StatefulSet.Namespace", statefulSet.Namespace, "StatefulSet.Name", statefulSet.Name)
			if err := r.Create(ctx, statefulSet); err != nil {
				log.Error(err, "Failed to create new StatefulSet", "statefulSet.Namespace", statefulSet.Namespace, "statefulSet.Name", statefulSet.Name)

				return ctrl.Result{}, err
			}
			if err := wait.Poll(time.Second*1, time.Second*15, func() (done bool, err error) {
				if err := r.Get(ctx, types.NamespacedName{Name: "cluster-" + instance.Name, Namespace: instance.Namespace}, foundcSS); err != nil {
					if errors.IsNotFound(err) {
						return false, nil
					} else {
						return false, err
					}
				}
				return true, nil
			}); err != nil {
				return ctrl.Result{}, err
			}
			// StatefulSet created successfully - return and requeue
			return ctrl.Result{Requeue: true}, nil
		}
		log.Error(err, "Failed to get StatefulSet")
	}

	// Check if the Service Account already exists, if not create a new one
	foundSA := &corev1.ServiceAccount{}
	if err := r.Get(ctx, types.NamespacedName{Name: "ipfs-cluster-" + instance.Name, Namespace: instance.Namespace}, foundSA); err != nil {
		if errors.IsNotFound(err) {
			// Define a new Service Account
			serviceAcct := r.saGenerate(instance)
			log.Info("Creating a new Service Account", "serviceAcct.Namespace", serviceAcct.Namespace, "serviceAcct.Name", serviceAcct.Name)
			if err := r.Create(ctx, serviceAcct); err != nil {
				log.Error(err, "Failed to create new Service Account", "serviceAcct.Namespace", serviceAcct.Namespace, "serviceAcct.Name", serviceAcct.Name)

				return ctrl.Result{}, err
			}
			if err := wait.Poll(time.Second*1, time.Second*15, func() (done bool, err error) {
				if err := r.Get(ctx, types.NamespacedName{Name: "ipfs-cluster-" + instance.Name, Namespace: instance.Namespace}, foundSA); err != nil {
					if errors.IsNotFound(err) {
						return false, nil
					} else {
						return false, err
					}
				}
				return true, nil
			}); err != nil {
				return ctrl.Result{}, err
			}
			// Service Account created successfully - return and requeue
			return ctrl.Result{Requeue: true}, nil
		}
		log.Error(err, "Failed to get Service Account")
	}

	// Check if the service already exists, if not create a new one
	foundService := &corev1.Service{}
	if err := r.Get(ctx, types.NamespacedName{Name: "cluster-" + instance.Name, Namespace: instance.Namespace}, foundService); err != nil {
		if errors.IsNotFound(err) {
			// Define a new service
			service := r.csvcGenerate(instance)
			log.Info("Creating a new Service", "service.Namespace", service.Namespace, "service.Name", service.Name)
			if err := r.Create(ctx, service); err != nil {
				log.Error(err, "Failed to create a Service", "service.Namespace", service.Namespace, "service.Name", service.Name)

				return ctrl.Result{}, err
			}
			if err := wait.Poll(time.Second*1, time.Second*15, func() (done bool, err error) {
				if err := r.Get(ctx, types.NamespacedName{Name: "cluster-" + instance.Name, Namespace: instance.Namespace}, foundService); err != nil {
					if errors.IsNotFound(err) {
						return false, nil
					} else {
						return false, err
					}
				}
				return true, nil
			}); err != nil {
				return ctrl.Result{}, err
			}
			// Service created successfully - return and requeue
			return ctrl.Result{Requeue: true}, nil
		}
		log.Error(err, "Failed to get Service")
	}

	// Check if the service already exists, if not create a new one
	if instance.Spec.Public {
		foundPublicService := &corev1.Service{}
		if err := r.Get(ctx, types.NamespacedName{Name: "public-gateway-" + instance.Name, Namespace: instance.Namespace}, foundPublicService); err != nil {
			if errors.IsNotFound(err) {
				// Define a new service
				service := r.pubSvcGenerate(instance)
				log.Info("Creating a new Service", "service.Namespace", service.Namespace, "service.Name", service.Name)
				if err := r.Create(ctx, service); err != nil {
					log.Error(err, "Failed to create a Service", "service.Namespace", service.Namespace, "service.Name", service.Name)

					return ctrl.Result{}, err
				}
				if err := wait.Poll(time.Second*1, time.Second*15, func() (done bool, err error) {
					if err := r.Get(ctx, types.NamespacedName{Name: "public-gateway-" + instance.Name, Namespace: instance.Namespace}, foundPublicService); err != nil {
						if errors.IsNotFound(err) {
							return false, nil
						} else {
							return false, err
						}
					}
					return true, nil
				}); err != nil {
					return ctrl.Result{}, err
				}
				// Service created successfully - return and requeue
				return ctrl.Result{Requeue: true}, nil
			}
			log.Error(err, "Failed to get Service")
		}
		if instance.Status.Address == "" {
			instance.Status.Address = getServiceAddress(foundPublicService)
			if err := r.Status().Update(ctx, instance); err != nil {
				log.Error(err, "LB may not be ready yet, requeuing")
				return ctrl.Result{Requeue: true}, nil
			}
		}
	}

	// Check for ingress
	foundIngress := &networkingv1.Ingress{}
	if err := r.Get(ctx, types.NamespacedName{Name: "ipfs-" + instance.Name, Namespace: instance.Namespace}, foundIngress); err != nil {
		if errors.IsNotFound(err) {
			// Define a new service
			ingress := r.ingressGenerate(instance)
			log.Info("Creating a new Ingress", "ingress.Namespace", ingress.Namespace, "ingress.Name", ingress.Name)
			if err := r.Create(ctx, ingress); err != nil {
				log.Error(err, "Failed to create a Ingress", "ingress.Namespace", ingress.Namespace, "ingress.Name", ingress.Name)

				return ctrl.Result{}, err
			}
			if err := wait.Poll(time.Second*1, time.Second*15, func() (done bool, err error) {
				if err := r.Get(ctx, types.NamespacedName{Name: "ipfs-" + instance.Name, Namespace: instance.Namespace}, foundIngress); err != nil {
					if errors.IsNotFound(err) {
						return false, nil
					} else {
						return false, err
					}
				}
				return true, nil
			}); err != nil {
				return ctrl.Result{}, err
			}
			// Service created successfully - return and requeue
			return ctrl.Result{Requeue: true}, nil
		}
		log.Error(err, "Failed to get Ingress")
	}

	// Check if the service already exists, if not create a new one
	foundipfsService := &corev1.Service{}
	if err := r.Get(ctx, types.NamespacedName{Name: "ipfs-cluster-" + instance.Name, Namespace: instance.Namespace}, foundipfsService); err != nil {
		if errors.IsNotFound(err) {
			// Define a new service
			service := r.isvcGenerate(instance)
			log.Info("Creating a new Service", "service.Namespace", service.Namespace, "service.Name", service.Name)
			if err := r.Create(ctx, service); err != nil {
				log.Error(err, "Failed to create a Service", "service.Namespace", service.Namespace, "service.Name", service.Name)

				return ctrl.Result{}, err
			}
			if err := wait.Poll(time.Second*1, time.Second*15, func() (done bool, err error) {
				if err := r.Get(ctx, types.NamespacedName{Name: "ipfs-cluster-" + instance.Name, Namespace: instance.Namespace}, foundService); err != nil {
					if errors.IsNotFound(err) {
						return false, nil
					} else {
						return false, err
					}
				}
				return true, nil
			}); err != nil {
				return ctrl.Result{}, err
			}
			// Service created successfully - return and requeue
			return ctrl.Result{Requeue: true}, nil
		}
		log.Error(err, "Failed to get Service")
	}
	return ctrl.Result{}, nil
}

func (r *IpfsReconciler) CleanUpOpjects(ctx context.Context, instance *clusterv1alpha1.Ipfs) error {
	// Delete all the objects that we created to make sure that things are removed.
	err := r.Delete(ctx, &appsv1.StatefulSet{ObjectMeta: metav1.ObjectMeta{Name: "ipfs-" + instance.Name, Namespace: instance.Namespace}}, client.PropagationPolicy(metav1.DeletePropagationBackground))
	if err != nil && !(errors.IsGone(err) || errors.IsNotFound(err)) {
		return err
	}

	err = r.Delete(ctx, &corev1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{Name: "ipfs-cluster-" + instance.Name, Namespace: instance.Namespace}})
	if err != nil && !(errors.IsGone(err) || errors.IsNotFound(err)) {
		return err
	}

	err = r.Delete(ctx, &corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: "ipfs-cluster-" + instance.Name, Namespace: instance.Namespace}})
	if err != nil && !(errors.IsGone(err) || errors.IsNotFound(err)) {
		return err
	}

	err = r.Delete(ctx, &corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: "cluster-0-ipfs-cluster-" + instance.Name, Namespace: instance.Namespace}})
	if err != nil && !(errors.IsGone(err) || errors.IsNotFound(err)) {
		return err
	}

	err = r.Delete(ctx, &corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: "public-gateway-" + instance.Name, Namespace: instance.Namespace}})
	if err != nil && !(errors.IsGone(err) || errors.IsNotFound(err)) {
		return err
	}

	err = r.Delete(ctx, &corev1.PersistentVolumeClaim{ObjectMeta: metav1.ObjectMeta{Name: "ipfs-storage-cluster-" + instance.Name, Namespace: instance.Namespace}})
	if err != nil && !(errors.IsGone(err) || errors.IsNotFound(err)) {
		return err
	}

	err = r.Delete(ctx, &corev1.PersistentVolumeClaim{ObjectMeta: metav1.ObjectMeta{Name: "ipfs-storage-ipfs-" + instance.Name, Namespace: instance.Namespace}})
	if err != nil && !(errors.IsGone(err) || errors.IsNotFound(err)) {
		return err
	}

	err = r.Delete(ctx, &appsv1.StatefulSet{ObjectMeta: metav1.ObjectMeta{Name: "cluster-" + instance.Name, Namespace: instance.Namespace}})
	if err != nil && !(errors.IsGone(err) || errors.IsNotFound(err)) {
		return err
	}

	return nil
}

func getServiceAddress(svc *corev1.Service) string {
	address := svc.Spec.ClusterIP
	if svc.Spec.Type == corev1.ServiceTypeLoadBalancer {
		if len(svc.Status.LoadBalancer.Ingress) > 0 {
			if svc.Status.LoadBalancer.Ingress[0].Hostname != "" {
				address = svc.Status.LoadBalancer.Ingress[0].Hostname
			} else if svc.Status.LoadBalancer.Ingress[0].IP != "" {
				address = svc.Status.LoadBalancer.Ingress[0].IP
			}
		} else {
			address = ""
		}
	}
	return address
}

func (r *IpfsReconciler) saGenerate(m *clusterv1alpha1.Ipfs) *corev1.ServiceAccount {
	// Define a new Service Account object
	serviceAcct := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "ipfs-cluster-" + m.Name,
			Namespace: m.Namespace,
		},
	}
	// Service Account reconcile finished
	ctrl.SetControllerReference(m, serviceAcct, r.Scheme)
	return serviceAcct
}

// Generate the statefulset object
func (r *IpfsReconciler) iSSGenerate(m *clusterv1alpha1.Ipfs) *appsv1.StatefulSet {
	// Define a new StatefulSet object
	replacas := int32(1)
	ss := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "ipfs-" + m.Name,
			Namespace: m.Namespace,
		},
		Spec: appsv1.StatefulSetSpec{
			Replicas: &replacas,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app.kubernetes.io/name":     "ipfs-cluster-" + m.Name,
					"app.kubernetes.io/instance": "ipfs-cluster-" + m.Name,
					"nodeType":                   "ipfs",
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app.kubernetes.io/name":     "ipfs-cluster-" + m.Name,
						"app.kubernetes.io/instance": "ipfs-cluster-" + m.Name,
						"nodeType":                   "ipfs",
					},
				},
				Spec: corev1.PodSpec{
					ServiceAccountName: "ipfs-cluster-" + m.Name,
					Containers: []corev1.Container{
						{
							Name:            "ipfs",
							Image:           "quay.io/redhat-et-ipfs/ipfs:v0.12.2",
							ImagePullPolicy: corev1.PullIfNotPresent,
							Env: []corev1.EnvVar{
								{
									Name:  "IPFS_PROFILE",
									Value: "flatfs",
								},
								{
									Name:  "IPFS_ADDITIONAL_ANNOUNCE",
									Value: m.Status.Address,
								},
							},
							Ports: []corev1.ContainerPort{
								{
									Name:          "swarm",
									ContainerPort: 4001,
									Protocol:      corev1.ProtocolTCP,
								},
								{
									Name:          "zeroconf",
									ContainerPort: 5353,
									Protocol:      corev1.ProtocolUDP,
								},
								{
									Name:          "api",
									ContainerPort: 5001,
									Protocol:      corev1.ProtocolTCP,
								},
								{
									Name:          "http",
									ContainerPort: 8080,
									Protocol:      corev1.ProtocolTCP,
								},
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "ipfs-storage",
									MountPath: "/data/ipfs",
								},
							},
						},
					},
				},
			},
			VolumeClaimTemplates: []corev1.PersistentVolumeClaim{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "ipfs-storage",
					},
					Spec: corev1.PersistentVolumeClaimSpec{
						AccessModes: []corev1.PersistentVolumeAccessMode{
							corev1.ReadWriteOnce,
						},
						Resources: corev1.ResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceStorage: resource.MustParse(m.Spec.IpfsStorage),
							},
						},
					},
				},
			},
			ServiceName:                          "ipfs-" + m.Name,
			PodManagementPolicy:                  "",
			UpdateStrategy:                       appsv1.StatefulSetUpdateStrategy{},
			RevisionHistoryLimit:                 new(int32),
			MinReadySeconds:                      0,
			PersistentVolumeClaimRetentionPolicy: &appsv1.StatefulSetPersistentVolumeClaimRetentionPolicy{},
		},
	}
	// StatefulSet reconcile finished
	ctrl.SetControllerReference(m, ss, r.Scheme)
	return ss
}

// Generate the statefulset object
func (r *IpfsReconciler) cSSGenerate(m *clusterv1alpha1.Ipfs) *appsv1.StatefulSet {
	replacas := int32(1)
	// Define a new StatefulSet object
	ss := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "cluster-" + m.Name,
			Namespace: m.Namespace,
		},
		Spec: appsv1.StatefulSetSpec{
			Replicas: &replacas,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app.kubernetes.io/name":     "ipfs-cluster-" + m.Name,
					"app.kubernetes.io/instance": "ipfs-cluster-" + m.Name,
					"nodeType":                   "cluster",
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app.kubernetes.io/name":     "ipfs-cluster-" + m.Name,
						"app.kubernetes.io/instance": "ipfs-cluster-" + m.Name,
						"nodeType":                   "cluster",
					},
				},
				Spec: corev1.PodSpec{
					ServiceAccountName: "ipfs-cluster-" + m.Name,
					Containers: []corev1.Container{
						{
							Name: "cluster-" + m.Name,
							Env: []corev1.EnvVar{
								{
									Name:  "CLUSTER_PEERNAME",
									Value: "cluster-0",
								},
								{
									Name:  "CLUSTER_IPFSHTTP_NODEMULTIADDRESS",
									Value: "/dns4/ipfs-cluster-" + m.Name + "." + m.Namespace + ".svc.cluster.local/tcp/5001",
								},
								{
									Name:  "CLUSTER_CRDT_TRUSTEDPEERS",
									Value: "*",
								},
								{
									Name:  "CLUSTER_RESTAPI_HTTPLISTENMULTIADDRESS",
									Value: "/ip4/0.0.0.0/tcp/9094",
								},
								{
									Name:  "CLUSTER_MONITORPINGINTERVAL",
									Value: "2s",
								},
							},
							Image:           "quay.io/redhat-et-ipfs/cluster:v1.0.0",
							ImagePullPolicy: corev1.PullIfNotPresent,
							Ports: []corev1.ContainerPort{
								{
									Name:          "p2p",
									ContainerPort: 9096,
									Protocol:      corev1.ProtocolTCP,
								},
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "ipfs-storage",
									MountPath: "/data/ipfs-cluster",
								},
							},
						},
					},
				},
			},
			VolumeClaimTemplates: []corev1.PersistentVolumeClaim{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "ipfs-storage",
					},
					Spec: corev1.PersistentVolumeClaimSpec{
						AccessModes: []corev1.PersistentVolumeAccessMode{
							corev1.ReadWriteOnce,
						},
						Resources: corev1.ResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceStorage: resource.MustParse(m.Spec.ClusterStorage),
							},
						},
					},
				},
			},
			ServiceName:                          "cluster-" + m.Name,
			PodManagementPolicy:                  "",
			UpdateStrategy:                       appsv1.StatefulSetUpdateStrategy{},
			RevisionHistoryLimit:                 new(int32),
			MinReadySeconds:                      0,
			PersistentVolumeClaimRetentionPolicy: &appsv1.StatefulSetPersistentVolumeClaimRetentionPolicy{},
		},
	}
	// StatefulSet reconcile finished
	ctrl.SetControllerReference(m, ss, r.Scheme)
	return ss
}

func (r *IpfsReconciler) ingressGenerate(m *clusterv1alpha1.Ipfs) *networkingv1.Ingress {
	PathTypeImplementationSpecific := networkingv1.PathType("ImplementationSpecific")
	ingress := &networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "ipfs-" + m.Name,
			Namespace: m.Namespace,
			Labels: map[string]string{
				"app.kubernetes.io/name":     "ipfs-cluster-" + m.Name,
				"app.kubernetes.io/instance": "ipfs-cluster-" + m.Name,
				"nodeType":                   "ipfs",
			},
		},
		Spec: networkingv1.IngressSpec{
			Rules: []networkingv1.IngressRule{
				{
					Host: "cluster-" + m.Namespace + "." + m.Spec.URL,
					IngressRuleValue: networkingv1.IngressRuleValue{
						HTTP: &networkingv1.HTTPIngressRuleValue{
							Paths: []networkingv1.HTTPIngressPath{
								{
									PathType: &PathTypeImplementationSpecific,

									Path: "/",
									Backend: networkingv1.IngressBackend{
										Service: &networkingv1.IngressServiceBackend{
											Name: "ipfs-cluster-" + m.Name,
											Port: networkingv1.ServiceBackendPort{
												Number: 8080,
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}
	// Service reconcile finished
	ctrl.SetControllerReference(m, ingress, r.Scheme)
	return ingress
}

func (r *IpfsReconciler) csvcGenerate(m *clusterv1alpha1.Ipfs) *corev1.Service {
	// Define a new service and generate secret
	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "cluster-" + m.Name,
			Namespace: m.Namespace,
			Labels: map[string]string{
				"app.kubernetes.io/name":     "ipfs-cluster-" + m.Name,
				"app.kubernetes.io/instance": "ipfs-cluster-" + m.Name,
				"nodeType":                   "cluster",
			},
		},
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{
				{
					Port: 9096,
					Name: "p2p",
				},
			},
			Selector: map[string]string{
				"app.kubernetes.io/name":     "ipfs-cluster-" + m.Name,
				"app.kubernetes.io/instance": "ipfs-cluster-" + m.Name,
				"nodeType":                   "cluster",
			},
		},
	}
	// Service reconcile finished
	ctrl.SetControllerReference(m, service, r.Scheme)
	return service
}

func (r *IpfsReconciler) isvcGenerate(m *clusterv1alpha1.Ipfs) *corev1.Service {
	// Define a new service and generate secret
	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "ipfs-cluster-" + m.Name,
			Namespace: m.Namespace,
			Labels: map[string]string{
				"app.kubernetes.io/name":     "ipfs-cluster-" + m.Name,
				"app.kubernetes.io/instance": "ipfs-cluster-" + m.Name,
				"nodeType":                   "ipfs",
			},
		},
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{
				{
					Port: 4001,
					Name: "swarm",
				},
				{
					Port: 5001,
					Name: "api",
				},
				{
					Port: 8080,
					Name: "gateway",
				},
			},
			Selector: map[string]string{
				"app.kubernetes.io/name":     "ipfs-cluster-" + m.Name,
				"app.kubernetes.io/instance": "ipfs-cluster-" + m.Name,
				"nodeType":                   "ipfs",
			},
		},
	}
	// Service reconcile finished
	ctrl.SetControllerReference(m, service, r.Scheme)
	return service
}

func (r *IpfsReconciler) pubSvcGenerate(m *clusterv1alpha1.Ipfs) *corev1.Service {
	// Define a new service and generate secret
	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "public-gateway-" + m.Name,
			Namespace: m.Namespace,
			Annotations: map[string]string{
				"service.beta.kubernetes.io/aws-load-balancer-type":   "nlb",
				"service.beta.kubernetes.io/aws-load-balancer-scheme": "internet-facing",
			},
			Labels: map[string]string{
				"app.kubernetes.io/name":     "ipfs-cluster-" + m.Name,
				"app.kubernetes.io/instance": "ipfs-cluster-" + m.Name,
				"nodeType":                   "ipfs",
			},
		},
		Spec: corev1.ServiceSpec{
			Type: corev1.ServiceTypeLoadBalancer,
			Ports: []corev1.ServicePort{
				{
					Port: 4001,
					Name: "swarm",
				},
			},
			Selector: map[string]string{
				"app.kubernetes.io/name":     "ipfs-cluster-" + m.Name,
				"app.kubernetes.io/instance": "ipfs-cluster-" + m.Name,
				"nodeType":                   "ipfs",
			},
		},
	}
	// Service reconcile finished
	ctrl.SetControllerReference(m, service, r.Scheme)
	return service
}

// SetupWithManager sets up the controller with the Manager.
func (r *IpfsReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&clusterv1alpha1.Ipfs{}).
		Owns(&appsv1.StatefulSet{}, builder.OnlyMetadata).
		Owns(&corev1.Service{}, builder.OnlyMetadata).
		Owns(&corev1.ServiceAccount{}, builder.OnlyMetadata).
		WithOptions(controller.Options{
			MaxConcurrentReconciles: 5,
		}).
		Complete(r)
}
