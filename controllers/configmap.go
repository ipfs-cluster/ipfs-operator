package controllers

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"

	clusterv1alpha1 "github.com/redhat-et/ipfs-operator/api/v1alpha1"
)

func (r *IpfsReconciler) configMapConfig(m *clusterv1alpha1.Ipfs, peerid string) (*corev1.ConfigMap, string) {
	cmName := "ipfs-cluster-" + m.Name
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cmName,
			Namespace: m.Namespace,
		},
		Data: map[string]string{
			"BOOTSTRAP_PEER_ID": peerid,
		},
	}
	ctrl.SetControllerReference(m, cm, r.Scheme)
	return cm, cmName
}
