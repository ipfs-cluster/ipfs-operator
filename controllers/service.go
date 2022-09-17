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
func (r *IpfsReconciler) serviceCluster(
	m *clusterv1alpha1.Ipfs,
	svc *corev1.Service,
) (controllerutil.MutateFn, string) {
	svcName := "ipfs-cluster-internal" + m.Name
	expected := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      svcName,
			Namespace: m.Namespace,
		},
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{
				{
					Name:       nameIpfsHttp,
					Protocol:   corev1.ProtocolTCP,
					Port:       portIpfsHttp,
					TargetPort: intstr.FromString(nameIpfsHttp),
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

// If enabled, IPFS gateway serivce
func (r *IpfsReconciler) serviceGateway(
	m *clusterv1alpha1.Ipfs,
	svc *corev1.Service,
) (controllerutil.MutateFn, string) {
	svcName := "ipfs-cluster-gateway" + m.Name
	expected := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:        svcName,
			Namespace:   m.Namespace,
			Annotations: m.Spec.Gateway.Annotations,
		},
		Spec: corev1.ServiceSpec{
			Type: "LoadBalancer",
			Ports: []corev1.ServicePort{
				{
					Name:       nameIpfsHttp,
					Protocol:   corev1.ProtocolTCP,
					Port:       portIpfsHttp,
					TargetPort: intstr.FromString(nameIpfsHttp),
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

// If enabled, the cluster API
func (r *IpfsReconciler) serviceAPI(
	m *clusterv1alpha1.Ipfs,
	svc *corev1.Service,
) (controllerutil.MutateFn, string) {
	svcName := "ipfs-cluster-internal" + m.Name
	expected := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:        svcName,
			Namespace:   m.Namespace,
			Annotations: m.Spec.ClusterAPI.Annotations,
		},
		Spec: corev1.ServiceSpec{
			Type: "LoadBalancer",
			Ports: []corev1.ServicePort{
				{
					Name:       nameClusterAPI,
					Protocol:   corev1.ProtocolTCP,
					Port:       portClusterAPI,
					TargetPort: intstr.FromString(nameClusterAPI),
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
