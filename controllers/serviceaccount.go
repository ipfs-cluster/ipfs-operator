package controllers

import (
	"context"
	"log"

	clusterv1alpha1 "github.com/redhat-et/ipfs-operator/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	ctrlutil "sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func (r *IpfsClusterReconciler) ensureSA(ctx context.Context, m *clusterv1alpha1.IpfsCluster) (*corev1.ServiceAccount,
	error) {
	log.Println("ensuring service account")
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
		log.Printf("could not create serviceOrUpdate service account: %s\n", err.Error())
		return nil, err
	}
	log.Println("completed operation:", res)
	return sa, nil
}
