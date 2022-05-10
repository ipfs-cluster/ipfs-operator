package controllers

import (
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	ctrl "sigs.k8s.io/controller-runtime"

	clusterv1alpha1 "github.com/redhat-et/ipfs-operator/api/v1alpha1"
)

func (r *IpfsReconciler) statefulSet(m *clusterv1alpha1.Ipfs,
	serviceName string,
	secretName string,
	configMapName string,
	configMapBootstrapScriptName string) *appsv1.StatefulSet {
	ssName := "ipfs-cluster-" + m.Name
	ss := &appsv1.StatefulSet{
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
							Image: "ipfs/go-ipfs:v0.12.2",
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
							Image:           "ipfs/go-ipfs:v0.12.2",
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
										Port: intstr.FromString("swarm"),
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
							},
							Resources: corev1.ResourceRequirements{},
						},
						{
							Name:            "ipfs-cluster",
							Image:           "ipfs/ipfs-cluster:v1.0.1",
							ImagePullPolicy: corev1.PullIfNotPresent,
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
									Name:  "IPFS_PROFILE",
									Value: "flatfs",
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
									Protocol:      corev1.ProtocolUDP,
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
										Port: intstr.FromString("cluster-swarm"),
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
	ctrl.SetControllerReference(m, ss, r.Scheme)
	return ss
}
