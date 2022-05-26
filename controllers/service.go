package controllers

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	ctrl "sigs.k8s.io/controller-runtime"

	clusterv1alpha1 "github.com/redhat-et/ipfs-operator/api/v1alpha1"
)

func (r *IpfsReconciler) serviceCluster(m *clusterv1alpha1.Ipfs) (*corev1.Service, string) {
	svcName := "ipfs-cluster-" + m.Name
	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      svcName,
			Namespace: m.Namespace,
			// TODO: annotations for external dns
		},
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{
				{
					Name:       "swarm",
					Protocol:   corev1.ProtocolTCP,
					Port:       4001,
					TargetPort: intstr.FromString("swarm"),
				},
				{
					Name:       "swarm-udp",
					Protocol:   corev1.ProtocolUDP,
					Port:       4002,
					TargetPort: intstr.FromString("swarm-udp"),
				},
				{
					Name:       "ws",
					Protocol:   corev1.ProtocolTCP,
					Port:       8081,
					TargetPort: intstr.FromString("ws"),
				},
				{
					Name:       "http",
					Protocol:   corev1.ProtocolTCP,
					Port:       8080,
					TargetPort: intstr.FromString("http"),
				},
				{
					Name:       "api-http",
					Protocol:   corev1.ProtocolTCP,
					Port:       9094,
					TargetPort: intstr.FromString("api-http"),
				},
				{
					Name:       "proxy-http",
					Protocol:   corev1.ProtocolTCP,
					Port:       9095,
					TargetPort: intstr.FromString("proxy-http"),
				},
				{
					Name:       "cluster-swarm",
					Protocol:   corev1.ProtocolTCP,
					Port:       9096,
					TargetPort: intstr.FromString("cluster-swarm"),
				},
			},
			Selector: map[string]string{
				"app.kubernetes.io/name": "ipfs-cluster-" + m.Name,
			},
		},
	}
	ctrl.SetControllerReference(m, svc, r.Scheme)
	return svc, svcName
}
