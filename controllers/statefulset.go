package controllers

import (
	"regexp"
	"strings"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	clusterv1alpha1 "github.com/redhat-et/ipfs-operator/api/v1alpha1"
)

// These declare constants for timeouts.
const (
	secondsPerMinute = 60
	tenSeconds       = 10
	thirtySeconds    = 30
)

// Defines port numbers to be used by the IPFS containers.
const (
	portAPIHTTP      = 9094
	portProxyHTTP    = 9095
	portClusterSwarm = 9096
	portSwarm        = 4001
	portSwarmUDP     = 4002
	portAPI          = 5001
	portPprof        = 6060
	portWS           = 8081
	portHTTP         = 8080
)

// Misclaneous constants.
const (
	// notDNSPattern Defines a ReGeX pattern to match non-DNS names.
	notDNSPattern = "[[:^alnum:]]"
	// ipfsClusterImage Defines which container image to use when pulling IPFS Cluster.
	// HACK: break this up so the version is parameterized, and we can inject the image locally.
	ipfsClusterImage = "docker.io/ipfs/ipfs-cluster:1.0.2"
	// ipfsClusterMountPath Defines where the cluster storage volume is mounted.
	ipfsClusterMountPath = "/data/ipfs-cluster"
	// ipfsMountPath Defines where the IPFS volume is mounted.
	ipfsMountPath = "/data/ipfs"
	// ipfsImage Defines which image we should pull when running IPFS containers.
	ipfsImage = "docker.io/ipfs/kubo:v0.14.0"
)

// statefulSet Returns a mutate function that creates a statefulSet for the
// given IPFS cluster.
// FIXME: break this function up to use createOrUpdate and set values in the struct line-by-line
//
//	instead of setting the entire thing all at once.
//
// nolint:funlen // Function is long due to Kube resource definitions
func (r *IpfsReconciler) statefulSet(m *clusterv1alpha1.IpfsCluster,
	sts *appsv1.StatefulSet,
	serviceName string,
	secretName string,
	configMapName string,
	configMapBootstrapScriptName string,
) controllerutil.MutateFn {
	ssName := "ipfs-cluster-" + m.Name

	expected := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      ssName,
			Namespace: m.Namespace,
		},
		Spec: appsv1.StatefulSetSpec{
			Replicas: &m.Spec.Replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app.kubernetes.io/name": ssName,
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app.kubernetes.io/name": ssName,
					},
				},
				Spec: corev1.PodSpec{
					ServiceAccountName: ssName,
					InitContainers: []corev1.Container{
						{
							Name:  "configure-ipfs",
							Image: ipfsImage,
							Command: []string{
								"sh",
								"/custom/configure-ipfs.sh",
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "ipfs-storage",
									MountPath: "/data/ipfs",
								},
								{
									Name:      "configure-script",
									MountPath: "custom",
								},
							},
						},
					},
					Containers: []corev1.Container{
						{
							Name:            "ipfs",
							Image:           ipfsImage,
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
									ContainerPort: portSwarm,
									Protocol:      corev1.ProtocolTCP,
								},
								{
									Name:          "swarm-udp",
									ContainerPort: portSwarmUDP,
									Protocol:      corev1.ProtocolUDP,
								},
								{
									Name:          "api",
									ContainerPort: portAPI,
									Protocol:      corev1.ProtocolTCP,
								},
								{
									Name:          "ws",
									ContainerPort: portWS,
									Protocol:      corev1.ProtocolTCP,
								},
								{
									Name:          "http",
									ContainerPort: portHTTP,
									Protocol:      corev1.ProtocolTCP,
								},
							},
							LivenessProbe: &corev1.Probe{
								ProbeHandler: corev1.ProbeHandler{
									TCPSocket: &corev1.TCPSocketAction{
										Port: intstr.FromString("swarm"),
									},
								},
								InitialDelaySeconds: thirtySeconds,
								TimeoutSeconds:      tenSeconds,
								PeriodSeconds:       secondsPerMinute,
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "ipfs-storage",
									MountPath: ipfsMountPath,
								},
							},
							Resources: corev1.ResourceRequirements{},
						},
						{
							Name:            "ipfs-cluster",
							Image:           ipfsClusterImage,
							ImagePullPolicy: corev1.PullIfNotPresent,
							Command: []string{
								"sh",
								"/custom/entrypoint.sh",
							},
							Args: []string{
								"run",
							},
							Env: []corev1.EnvVar{
								{
									Name: "BOOTSTRAP_PEER_ID",
									ValueFrom: &corev1.EnvVarSource{
										ConfigMapKeyRef: &corev1.ConfigMapKeySelector{
											LocalObjectReference: corev1.LocalObjectReference{
												Name: configMapName,
											},
											Key: "BOOTSTRAP_PEER_ID",
										},
									},
								},
								{
									Name: "BOOTSTRAP_PEER_PRIV_KEY",
									ValueFrom: &corev1.EnvVarSource{
										SecretKeyRef: &corev1.SecretKeySelector{
											LocalObjectReference: corev1.LocalObjectReference{
												Name: secretName,
											},
											Key: "BOOTSTRAP_PEER_PRIV_KEY",
										},
									},
								},
								{
									Name: "CLUSTER_SECRET",
									ValueFrom: &corev1.EnvVarSource{
										SecretKeyRef: &corev1.SecretKeySelector{
											LocalObjectReference: corev1.LocalObjectReference{
												Name: secretName,
											},
											Key: "CLUSTER_SECRET",
										},
									},
								},
								{
									Name:  "CLUSTER_MONITOR_PING_INTERVAL",
									Value: "3m",
								},
								{
									Name:  "SVC_NAME",
									Value: serviceName,
								},
							},
							Ports: []corev1.ContainerPort{
								{
									Name:          "api-http",
									ContainerPort: portAPIHTTP,
									Protocol:      corev1.ProtocolTCP,
								},
								{
									Name:          "proxy-http",
									ContainerPort: portProxyHTTP,
									Protocol:      corev1.ProtocolUDP,
								},
								{
									Name:          "cluster-swarm",
									ContainerPort: portClusterSwarm,
									Protocol:      corev1.ProtocolTCP,
								},
							},
							LivenessProbe: &corev1.Probe{
								ProbeHandler: corev1.ProbeHandler{
									TCPSocket: &corev1.TCPSocketAction{
										Port: intstr.FromString("cluster-swarm"),
									},
								},
								InitialDelaySeconds: thirtySeconds,
								TimeoutSeconds:      tenSeconds,
								PeriodSeconds:       secondsPerMinute,
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "cluster-storage",
									MountPath: ipfsClusterMountPath,
								},
								{
									Name:      "configure-script",
									MountPath: "custom",
								},
							},
							Resources: corev1.ResourceRequirements{},
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: "configure-script",
							VolumeSource: corev1.VolumeSource{
								ConfigMap: &corev1.ConfigMapVolumeSource{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: configMapBootstrapScriptName,
									},
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
								corev1.ResourceStorage: resource.MustParse(m.Spec.ClusterStorage),
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
								corev1.ResourceStorage: resource.MustParse(m.Spec.IpfsStorage),
							},
						},
					},
				},
			},
			ServiceName: serviceName,
		},
	}

	// Add a follower container for each follow.
	follows := followContainers(m)
	expected.Spec.Template.Spec.Containers = append(expected.Spec.Template.Spec.Containers, follows...)
	expected.DeepCopyInto(sts)
	// FIXME: catch this error before returning a function that just errors
	if err := ctrl.SetControllerReference(m, sts, r.Scheme); err != nil {
		return func() error { return err }
	}
	return func() error {
		sts.Spec = expected.Spec
		return nil
	}
}

// followContainers Returns a list of container objects which follow the given followParams.
func followContainers(m *clusterv1alpha1.IpfsCluster) []corev1.Container {
	// objects need to be RFC-1123 compliant, and k8s uses this regex to test.
	// https://github.com/kubernetes/apimachinery/blob/v0.24.2/pkg/util/validation/validation.go
	// dns1123LabelFmt "[a-z0-9]([-a-z0-9]*[a-z0-9])?"
	// We want to match the opposite.
	var notdns = regexp.MustCompile(notDNSPattern)
	containers := make([]corev1.Container, 0)
	for _, follow := range m.Spec.Follows {
		container := corev1.Container{
			Name:            "ipfs-cluster-follow-" + notdns.ReplaceAllString(strings.ToLower(follow.Name), "-"),
			Image:           ipfsClusterImage,
			ImagePullPolicy: corev1.PullIfNotPresent,
			Command: []string{
				"ipfs-cluster-follow",
				follow.Name,
				"run",
				"--init",
				follow.Template,
			},
			VolumeMounts: []corev1.VolumeMount{
				{
					Name:      "cluster-storage",
					MountPath: ipfsClusterMountPath,
				},
			},
		}
		containers = append(containers, container)
	}
	return containers
}
