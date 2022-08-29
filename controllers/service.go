package controllers

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	clusterv1alpha1 "github.com/redhat-et/ipfs-operator/api/v1alpha1"
)

func (r *IpfsReconciler) serviceCluster(
	m *clusterv1alpha1.IpfsCluster,
	svc *corev1.Service,
) (controllerutil.MutateFn, string) {
	svcName := "ipfs-cluster-" + m.Name
	expected := &corev1.Service{
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
					Port:       portSwarm,
					TargetPort: intstr.FromString("swarm"),
				},
				{
					Name:       "swarm-udp",
					Protocol:   corev1.ProtocolUDP,
					Port:       portSwarmUDP,
					TargetPort: intstr.FromString("swarm-udp"),
				},
				{
					Name:       "ws",
					Protocol:   corev1.ProtocolTCP,
					Port:       portWS,
					TargetPort: intstr.FromString("ws"),
				},
				{
					Name:       "http",
					Protocol:   corev1.ProtocolTCP,
					Port:       portHTTP,
					TargetPort: intstr.FromString("http"),
				},
				{
					Name:       "api-http",
					Protocol:   corev1.ProtocolTCP,
					Port:       portAPIHTTP,
					TargetPort: intstr.FromString("api-http"),
				},
				{
					Name:       "proxy-http",
					Protocol:   corev1.ProtocolTCP,
					Port:       portProxyHTTP,
					TargetPort: intstr.FromString("proxy-http"),
				},
				{
					Name:       "cluster-swarm",
					Protocol:   corev1.ProtocolTCP,
					Port:       portClusterSwarm,
					TargetPort: intstr.FromString("cluster-swarm"),
				},
			},
			Selector: map[string]string{
				"app.kubernetes.io/name": "ipfs-cluster-" + m.Name,
			},
		},
	}
	expected.DeepCopyInto(svc)
	// FIXME: catch this error before we run the function being returned
	if err := ctrl.SetControllerReference(m, svc, r.Scheme); err != nil {
		return func() error { return err }, ""
	}
	return func() error {
		svc.Spec = expected.Spec
		return nil
	}, svcName
}
