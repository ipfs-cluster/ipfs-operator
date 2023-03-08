package controllers

import (
	"context"

	"sigs.k8s.io/controller-runtime/pkg/log"

	clusterv1alpha1 "github.com/redhat-et/ipfs-operator/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	ctrlutil "sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func (r *IpfsClusterReconciler) ensureSA(ctx context.Context, m *clusterv1alpha1.IpfsCluster) (*corev1.ServiceAccount,
	error) {
	logger := log.FromContext(ctx)
	logger.Info("ensuring service account")
	// Define a new Service Account object
	sa := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "ipfs-cluster-" + m.Name,
			Namespace: m.Namespace,
		},
	}
	res, err := ctrlutil.CreateOrUpdate(ctx, r.Client, sa, func() error {
		if err := ctrl.SetControllerReference(m, sa, r.Scheme); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		logger.Error(err, "failed to create serviceaccount")
		return nil, err
	}
	logger.Info("created serviceaccount", "result", res)
	return sa, nil
}
