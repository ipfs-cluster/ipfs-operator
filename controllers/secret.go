package controllers

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"

	clusterv1alpha1 "github.com/redhat-et/ipfs-operator/api/v1alpha1"
)

func (r *IpfsReconciler) secretConfig(m *clusterv1alpha1.Ipfs, clusterSecret, bootstrapPrivateKey []byte) (*corev1.Secret, string) {
	secName := "ipfs-cluster-" + m.Name
	sec := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secName,
			Namespace: m.Namespace,
		},
		Data: map[string][]byte{
			"CLUSTER_SECRET":          clusterSecret,
			"BOOTSTRAP_PEER_PRIV_KEY": bootstrapPrivateKey,
		},
	}
	ctrl.SetControllerReference(m, sec, r.Scheme)
	return sec, secName
}
