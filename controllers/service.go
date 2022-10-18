package controllers

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	clusterv1alpha1 "github.com/redhat-et/ipfs-operator/api/v1alpha1"
)

// This is an internal service. It exposes the API and gateway ports
func (r *IpfsClusterReconciler) serviceCluster(
	m *clusterv1alpha1.IpfsCluster,
	svc *corev1.Service,
) (controllerutil.MutateFn, string) {
	svcName := "ipfs-cluster-internal-" + m.Name
	expected := expectedService(
		svcName,
		m.Name,
		m.Namespace,
		"ClusterIP",
		m.Annotations,
		[]corev1.ServicePort{
			{
				Name:       nameIpfsHTTP,
				Protocol:   corev1.ProtocolTCP,
				Port:       portIpfsHTTP,
				TargetPort: intstr.FromString(nameIpfsHTTP),
			},
			{
				Name:       nameClusterAPI,
				Protocol:   corev1.ProtocolTCP,
				Port:       portClusterAPI,
				TargetPort: intstr.FromString(nameClusterAPI),
			},
			{
				Name:       nameClusterProxy,
				Protocol:   corev1.ProtocolTCP,
				Port:       portClusterProxy,
				TargetPort: intstr.FromString(nameClusterProxy),
			},
		},
	)

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

// If enabled, IPFS gateway serivce
func (r *IpfsClusterReconciler) serviceGateway(
	m *clusterv1alpha1.IpfsCluster,
	svc *corev1.Service,
) (controllerutil.MutateFn, string) {
	svcName := "ipfs-cluster-gateway-" + m.Name
	expected := expectedService(
		svcName,
		m.Name,
		m.Namespace,
		"LoadBalancer",
		m.Annotations,
		[]corev1.ServicePort{
			{
				Name:       nameIpfsHTTP,
				Protocol:   corev1.ProtocolTCP,
				Port:       portIpfsHTTP,
				TargetPort: intstr.FromString(nameIpfsHTTP),
			},
		},
	)
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

// If enabled, the cluster API
func (r *IpfsClusterReconciler) serviceAPI(
	m *clusterv1alpha1.IpfsCluster,
	svc *corev1.Service,
) (controllerutil.MutateFn, string) {
	svcName := "ipfs-cluster-api-" + m.Name
	expected := expectedService(
		svcName,
		m.Name,
		m.Namespace,
		"LoadBalancer",
		m.Annotations,
		[]corev1.ServicePort{
			{
				Name:       nameClusterAPI,
				Protocol:   corev1.ProtocolTCP,
				Port:       portClusterAPI,
				TargetPort: intstr.FromString(nameClusterAPI),
			},
		},
	)
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

func expectedService(svcName,
	relName,
	namespace string,
	svcType corev1.ServiceType,
	annotations map[string]string,
	ports []corev1.ServicePort) *corev1.Service {
	expected := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:        svcName,
			Namespace:   namespace,
			Annotations: annotations,
		},
		Spec: corev1.ServiceSpec{
			Type:  svcType,
			Ports: ports,
			Selector: map[string]string{
				"app.kubernetes.io/name": "ipfs-cluster-" + relName,
			},
		},
	}
	return expected
}
