package controllers

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	clusterv1alpha1 "github.com/redhat-et/ipfs-operator/api/v1alpha1"
)

func (r *IpfsReconciler) configMapConfig(m *clusterv1alpha1.Ipfs, cm *corev1.ConfigMap, peerid string) (controllerutil.MutateFn, string) {
	cmName := "ipfs-cluster-" + m.Name
	expected := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cmName,
			Namespace: m.Namespace,
		},
		Data: map[string]string{
			"BOOTSTRAP_PEER_ID": peerid,
		},
	}
	expected.DeepCopyInto(cm)
	// FIXME: catch this error before we run the function being returned
	if err := ctrl.SetControllerReference(m, cm, r.Scheme); err != nil {
		return func() error { return err }, ""
	}
	return func() error {
		return nil
	}, cmName
}
