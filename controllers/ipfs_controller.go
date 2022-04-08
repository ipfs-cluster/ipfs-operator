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
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
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
//+kubebuilder:rbac:groups=batch,resources=jobs,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=core,resources=serviceaccounts,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=core,resources=services,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=apps,resources=statefulsets,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=core,resources=secrets,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=core,resources=persistentvolumeclaims,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=clusterroles,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=clusterrolebindings,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups="",resources=users,verbs=impersonate
//+kubebuilder:rbac:groups=route.openshift.io,resources=routes,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=networking.k8s.io,resources=networkpolicies,verbs=get;list;watch;create;update;patch;delete
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
	foundSS := &appsv1.StatefulSet{}
	if err := r.Get(ctx, types.NamespacedName{Name: "ipfs-cluster-" + instance.Name, Namespace: instance.Namespace}, foundSS); err != nil {
		if errors.IsNotFound(err) {
			// Define a new statefulset
			statefulSet := r.ssGenerate(instance)
			log.Info("Creating a new StatefulSet", "StatefulSet.Namespace", statefulSet.Namespace, "StatefulSet.Name", statefulSet.Name)
			if err := r.Create(ctx, statefulSet); err != nil {
				log.Error(err, "Failed to create new StatefulSet", "statefulSet.Namespace", statefulSet.Namespace, "statefulSet.Name", statefulSet.Name)

				return ctrl.Result{}, err
			}
			if err := wait.Poll(time.Second*1, time.Second*15, func() (done bool, err error) {
				if err := r.Get(ctx, types.NamespacedName{Name: "ipfs-cluster-" + instance.Name, Namespace: instance.Namespace}, foundSS); err != nil {
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

	// Check if configmap already exists, if not create a new one
	foundConfigMap := &corev1.ConfigMap{}
	if err := r.Get(ctx, types.NamespacedName{Name: "ipfs-cluster-" + instance.Name, Namespace: instance.Namespace}, foundConfigMap); err != nil {
		if errors.IsNotFound(err) {
			// Define a new configmap
			configMap := r.cmGenerate(instance)
			log.Info("Creating a new ConfigMap", "configMap.Namespace", configMap.Namespace, "configMap.Name", configMap.Name)
			if err := r.Create(ctx, configMap); err != nil {
				log.Error(err, "Failed to create a ConfigMap", "configMap.Namespace", configMap.Namespace, "configMap.Name", configMap.Name)

				return ctrl.Result{}, err
			}
			if err := wait.Poll(time.Second*1, time.Second*15, func() (done bool, err error) {
				if err := r.Get(ctx, types.NamespacedName{Name: "ipfs-cluster-" + instance.Name, Namespace: instance.Namespace}, foundConfigMap); err != nil {
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
			// Configmap created successfully - return and requeue
			return ctrl.Result{Requeue: true}, nil
		}
		log.Error(err, "Failed to get ConfigMap")
	}

	// Check if secret already exists, if not create a new one
	foundSecret := &corev1.Secret{}
	if err := r.Get(ctx, types.NamespacedName{Name: "ipfs-cluster-" + instance.Name, Namespace: instance.Namespace}, foundSecret); err != nil {
		if errors.IsNotFound(err) {
			// Define a new secret
			secret := r.secretGenerate(instance)
			log.Info("Creating a new Secret", "secret.Namespace", secret.Namespace, "secret.Name", secret.Name)
			if err := r.Create(ctx, secret); err != nil {
				log.Error(err, "Failed to create a Secret", "secret.Namespace", secret.Namespace, "secret.Name", secret.Name)

				return ctrl.Result{}, err
			}
			if err := wait.Poll(time.Second*1, time.Second*15, func() (done bool, err error) {
				if err := r.Get(ctx, types.NamespacedName{Name: "ipfs-cluster-" + instance.Name, Namespace: instance.Namespace}, foundSecret); err != nil {
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
			// Secret created successfully - return and requeue
			return ctrl.Result{Requeue: true}, nil
		}
		log.Error(err, "Failed to get Secret")
	}

	// Check if the service already exists, if not create a new one
	foundService := &corev1.Service{}
	if err := r.Get(ctx, types.NamespacedName{Name: "ipfs-cluster-" + instance.Name, Namespace: instance.Namespace}, foundService); err != nil {
		if errors.IsNotFound(err) {
			// Define a new service
			service := r.svcGenerate(instance)
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
	err := r.Delete(ctx, &batchv1.Job{ObjectMeta: metav1.ObjectMeta{Name: "ipfs-cluster-" + instance.Name, Namespace: instance.Namespace}}, client.PropagationPolicy(metav1.DeletePropagationBackground))
	if err != nil && !(errors.IsGone(err) || errors.IsNotFound(err)) {
		return err
	}

	err = r.Delete(ctx, &corev1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{Name: "ipfs-cluster-" + instance.Name, Namespace: instance.Namespace}})
	if err != nil && !(errors.IsGone(err) || errors.IsNotFound(err)) {
		return err
	}

	err = r.Delete(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "ipfs-cluster-" + instance.Name, Namespace: instance.Namespace}})
	if err != nil && !(errors.IsGone(err) || errors.IsNotFound(err)) {
		return err
	}

	err = r.Delete(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "ipfs-cluster-" + instance.Name, Namespace: instance.Namespace}})
	if err != nil && !(errors.IsGone(err) || errors.IsNotFound(err)) {
		return err
	}

	err = r.Delete(ctx, &appsv1.StatefulSet{ObjectMeta: metav1.ObjectMeta{Name: "ipfs-cluster-" + instance.Name, Namespace: instance.Namespace}})
	if err != nil && !(errors.IsGone(err) || errors.IsNotFound(err)) {
		return err
	}

	err = r.Delete(ctx, &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "ipfs-cluster-" + instance.Name, Namespace: instance.Namespace}})
	if err != nil && !(errors.IsGone(err) || errors.IsNotFound(err)) {
		return err
	}

	err = r.Delete(ctx, &corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: "ipfs-cluster-" + instance.Name, Namespace: instance.Namespace}})
	if err != nil && !(errors.IsGone(err) || errors.IsNotFound(err)) {
		return err
	}

	return nil
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
func (r *IpfsReconciler) cmGenerate(m *clusterv1alpha1.Ipfs) *corev1.ConfigMap {
	// Define a new ConfigMap object
	configMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "ipfs-cluster-" + m.Name,
			Namespace: m.Namespace,
		},
		Data: map[string]string{
			"bootstrap-peer-id": "blah",
		},
	}
	// ConfigMap reconcile finished
	ctrl.SetControllerReference(m, configMap, r.Scheme)
	return configMap
}

// Generate the secret for the ipfs cluster
func (r *IpfsReconciler) secretGenerate(m *clusterv1alpha1.Ipfs) *corev1.Secret {
	// Define a new Secret object
	secret := &corev1.Secret{
		TypeMeta:   metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{Name: "ipfs-cluster-" + m.Name, Namespace: m.Namespace},
		Immutable:  new(bool),
		Data: map[string][]byte{
			"cluster-secret":          []byte("blah"),
			"bootstrap-peer-priv-key": []byte("blah"),
		},
		StringData: map[string]string{},
		Type:       corev1.SecretTypeOpaque,
	}
	// Secret reconcile finished
	ctrl.SetControllerReference(m, secret, r.Scheme)
	return secret
}

// Generate the statefulset object
func (r *IpfsReconciler) ssGenerate(m *clusterv1alpha1.Ipfs) *appsv1.StatefulSet {
	// Define a new StatefulSet object
	ss := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "ipfs-cluster-" + m.Name,
			Namespace: m.Namespace,
		},
		Spec: appsv1.StatefulSetSpec{
			Replicas: &m.Spec.Replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": "ipfs-cluster-" + m.Name,
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app": "ipfs-cluster-" + m.Name,
					},
				},
				Spec: corev1.PodSpec{
					InitContainers: []corev1.Container{
						{
							Name:  "configure-ipfs",
							Image: "ipfs/go-ipfs:v0.4.18",
							Command: []string{
								"/bin/sh",
								"/custom/configure-ipfs.sh",
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "ipfs-storage",
									MountPath: "/data/ipfs",
								},
								{
									Name:      "configure-script",
									MountPath: "/custom",
								},
							},
						},
					},
					Containers: []corev1.Container{
						{
							Name:            "ipfs",
							Image:           "ipfs/go-ipfs:v0.4.18",
							ImagePullPolicy: corev1.PullIfNotPresent,
							Env: []corev1.EnvVar{
								{
									Name:  "IPFS_FD_MAX",
									Value: "4096",
								},
							},
							Ports: []corev1.ContainerPort{
								{
									Name:          "swarm",
									ContainerPort: 4001,
									Protocol:      corev1.ProtocolTCP,
								},
								{
									Name:          "swarm-udp",
									ContainerPort: 4002,
									Protocol:      corev1.ProtocolUDP,
								},
								{
									Name:          "api",
									ContainerPort: 5001,
									Protocol:      corev1.ProtocolTCP,
								},
								{
									Name:          "ws",
									ContainerPort: 8081,
									Protocol:      corev1.ProtocolTCP,
								},
								{
									Name:          "http",
									ContainerPort: 8080,
									Protocol:      corev1.ProtocolTCP,
								},
							},
							LivenessProbe: &corev1.Probe{
								ProbeHandler: corev1.ProbeHandler{
									TCPSocket: &corev1.TCPSocketAction{
										Port: intstr.IntOrString{
											StrVal: "swarm",
										},
									},
								},
								InitialDelaySeconds: 30,
								TimeoutSeconds:      5,
								PeriodSeconds:       15,
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "ipfs-storage",
									MountPath: "/data/ipfs",
								},
								{
									Name:      "configure-script",
									MountPath: "/custom",
								},
							},
						},
						{
							Name:            "ipfs-cluster-" + m.Name,
							Image:           "ipfs/go-ipfs:v0.4.18",
							ImagePullPolicy: corev1.PullIfNotPresent,
							Command: []string{
								"/bin/sh",
								"/custom/entrypoint.sh",
							},
							EnvFrom: []corev1.EnvFromSource{
								{
									ConfigMapRef: &corev1.ConfigMapEnvSource{
										LocalObjectReference: corev1.LocalObjectReference{
											Name: "env-config",
										},
									},
								},
							},
							Env: []corev1.EnvVar{
								{
									Name: "BOOTSTRAP_PEER_ID",
									ValueFrom: &corev1.EnvVarSource{
										ConfigMapKeyRef: &corev1.ConfigMapKeySelector{
											LocalObjectReference: corev1.LocalObjectReference{
												Name: "env-config",
											},
											Key: "bootstrap-peer-id",
										},
									},
								},
								{
									Name: "BOOTSTRAP_PEER_PRIV_KEY",
									ValueFrom: &corev1.EnvVarSource{
										SecretKeyRef: &corev1.SecretKeySelector{
											LocalObjectReference: corev1.LocalObjectReference{
												Name: "secret-config",
											},
											Key: "bootstrap-peer-priv-key",
										},
									},
								},
								{
									Name: "ClusterSecret",
									ValueFrom: &corev1.EnvVarSource{
										SecretKeyRef: &corev1.SecretKeySelector{
											LocalObjectReference: corev1.LocalObjectReference{
												Name: "secret-config",
											},
											Key: "cluster-secret",
										},
									},
								},
							},
							Ports: []corev1.ContainerPort{
								{
									Name:          "api-http",
									ContainerPort: 9094,
									Protocol:      corev1.ProtocolTCP,
								},
								{
									Name:          "proxy-http",
									ContainerPort: 9095,
									Protocol:      corev1.ProtocolTCP,
								},
								{
									Name:          "cluster-swarm",
									ContainerPort: 9096,
									Protocol:      corev1.ProtocolTCP,
								},
							},
							LivenessProbe: &corev1.Probe{
								ProbeHandler: corev1.ProbeHandler{
									TCPSocket: &corev1.TCPSocketAction{
										Port: intstr.IntOrString{
											StrVal: "cluster-swarm",
										},
									},
								},
								InitialDelaySeconds: 5,
								TimeoutSeconds:      5,
								PeriodSeconds:       10,
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "cluster-storage",
									MountPath: "/data/ipfs-cluster",
								},
								{
									Name:      "configure-script",
									MountPath: "/custom",
								},
							},
						},
					},
				},
			},
			VolumeClaimTemplates: []corev1.PersistentVolumeClaim{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "cluster-storage",
					},
					Spec: corev1.PersistentVolumeClaimSpec{
						AccessModes: []corev1.PersistentVolumeAccessMode{
							corev1.ReadWriteOnce,
						},
						Resources: corev1.ResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceStorage: resource.MustParse("1Gi"),
							},
						},
					},
				},
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
								corev1.ResourceStorage: resource.MustParse("1Gi"),
							},
						},
					},
				},
			},
			ServiceName:                          "ipfs-cluster-" + m.Name,
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

func (r *IpfsReconciler) svcGenerate(m *clusterv1alpha1.Ipfs) *corev1.Service {
	// Define a new service and generate secret
	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "ipfs-cluster-" + m.Name,
			Namespace: m.Namespace,
		},
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{
				{
					Port: 8080,
					Name: "primer",
				},
				{
					Port: 8888,
					Name: "oauth-proxy",
				},
			},
			Selector: map[string]string{
				"app.kubernetes.io/name":      "ipfs-cluster-" + m.Name,
				"app.kubernetes.io/component": "ipfs-cluster-" + m.Name,
				"app.kubernetes.io/part-of":   "ipfs-cluster-" + m.Name,
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
		Owns(&batchv1.Job{}, builder.OnlyMetadata).
		Owns(&appsv1.Deployment{}, builder.OnlyMetadata).
		WithOptions(controller.Options{
			MaxConcurrentReconciles: 5,
		}).
		Complete(r)
}
