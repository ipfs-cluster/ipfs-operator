package controllers

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	clusterv1alpha1 "github.com/redhat-et/ipfs-operator/api/v1alpha1"
)

func (r *IpfsReconciler) secretConfig(m *clusterv1alpha1.Ipfs, sec *corev1.Secret, clusterSecret, bootstrapPrivateKey []byte) (controllerutil.MutateFn, string) {
	secName := "ipfs-cluster-" + m.Name
	expected := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secName,
			Namespace: m.Namespace,
		},
		Data: map[string][]byte{
			"CLUSTER_SECRET":          clusterSecret,
			"BOOTSTRAP_PEER_PRIV_KEY": bootstrapPrivateKey,
		},
	}
	expected.DeepCopyInto(sec)
	// FIXME: catch this error before we run the function being returned
	if err := ctrl.SetControllerReference(m, sec, r.Scheme); err != nil {
		return func() error { return err }, ""
	}
	return func() error {
		return nil
	}, secName
}
