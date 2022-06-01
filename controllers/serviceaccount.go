package controllers

import (
	clusterv1alpha1 "github.com/redhat-et/ipfs-operator/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func (r *IpfsReconciler) serviceAccount(m *clusterv1alpha1.Ipfs, sa *corev1.ServiceAccount) controllerutil.MutateFn {
	// Define a new Service Account object
	expected := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "ipfs-cluster-" + m.Name,
			Namespace: m.Namespace,
		},
	}
	expected.DeepCopyInto(sa)
	ctrl.SetControllerReference(m, sa, r.Scheme)
	return func() error {
		return nil
	}
}
