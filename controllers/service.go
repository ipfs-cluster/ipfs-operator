package controllers

import (
	"context"
	"fmt"
	"log"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	ctrl "sigs.k8s.io/controller-runtime"

	clusterv1alpha1 "github.com/redhat-et/ipfs-operator/api/v1alpha1"
)

// ensureServiceCluster Returns the existing IPFS cluster service object or an error.
func (r *IpfsClusterReconciler) ensureServiceCluster(
	ctx context.Context,
	m *clusterv1alpha1.IpfsCluster,
) (*corev1.Service, error) {
	svcName := "ipfs-cluster-" + m.Name
	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      svcName,
			Namespace: m.Namespace,
			// TODO: annotations for external dns
		},
	}

	log.Println("creating or updating svc")
	op, err := ctrl.CreateOrUpdate(ctx, r.Client, svc, func() error {
		svc.Spec = corev1.ServiceSpec{}
		svc.Spec.Ports = []corev1.ServicePort{
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
		}
		svc.Spec.Selector = map[string]string{
			"app.kubernetes.io/name": "ipfs-cluster-" + m.Name,
		}
		if err := ctrl.SetControllerReference(m, svc, r.Scheme); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		log.Printf("failed on operation %s\n", op)
		return nil, fmt.Errorf("failed to create service: %w", err)
	}
	fmt.Printf("completed operation: %s\n", op)
	return svc, nil
}
