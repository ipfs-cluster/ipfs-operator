package controllers

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	peer "github.com/libp2p/go-libp2p/core/peer"
	clusterv1alpha1 "github.com/redhat-et/ipfs-operator/api/v1alpha1"
	"github.com/redhat-et/ipfs-operator/controllers/utils"
)

const (
	peerIDPrefix     = "peerID-"
	privateKeyPrefix = "privateKey-"
)

func (r *IpfsClusterReconciler) EnsureSecretConfig(
	ctx context.Context,
	m *clusterv1alpha1.IpfsCluster,
) (*corev1.Secret, error) {
	secName := "ipfs-cluster-" + m.Name

	expectedSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secName,
			Namespace: m.Namespace,
		},
	}
	// find secret
	err := r.Get(ctx, client.ObjectKeyFromObject(expectedSecret), expectedSecret)
	if err != nil {
		// test for unhandled errors
		if !errors.IsNotFound(err) {
			return nil, fmt.Errorf("could not get secret: %w", err)
		}
		err = r.createNewSecret(ctx, m, expectedSecret)
		if err != nil {
			return nil, fmt.Errorf("could not create new secret: %w", err)
		}
		return expectedSecret, nil
	}
	// secret exists.
	// test if we need to add more identities
	op, err := ctrl.CreateOrUpdate(ctx, r.Client, expectedSecret, func() error {
		numIdentities := countIdentities(expectedSecret)
		if numIdentities != m.Spec.Replicas {
			// create more identities if needed, otherwise they will be reused
			// when scaling down and then up again
			if numIdentities < m.Spec.Replicas {
				// create more
				err = generateNewIdentities(expectedSecret, numIdentities, m.Spec.Replicas)
				if err != nil {
					return fmt.Errorf("could not generate more identities: %w", err)
				}
			}
		}
		if ctrlErr := ctrl.SetControllerReference(m, expectedSecret, r.Scheme); ctrlErr != nil {
			return ctrlErr
		}
		return nil
	})
	fmt.Printf("completed operation %q on secret\n", op)
	if err != nil {
		fmt.Printf("could not update secret on operation: %q\n", op)
		return nil, fmt.Errorf("could not update secret: %w", err)
	}
	fmt.Printf("succeeded updating secret on operation %q\n", op)
	return expectedSecret, nil
}

// createNewSecret Attempts to create an entirely new secret containing information relevant to IPFS Cluster.
func (r *IpfsClusterReconciler) createNewSecret(ctx context.Context, m *clusterv1alpha1.IpfsCluster,
	secret *corev1.Secret) (err error) {
	// secret is not found.
	// initialize new secret
	secret.Data = make(map[string][]byte, 0)
	var clusterSecret, bootstrapPrivateKey string
	var peerID peer.ID

	// save data in secret
	if clusterSecret, err = utils.NewClusterSecret(); err != nil {
		return fmt.Errorf("could not create cluster secret: %w", err)
	}
	if peerID, bootstrapPrivateKey, err = utils.GenerateIdentity(); err != nil {
		return fmt.Errorf("could not create new ipfs identity: %w", err)
	}

	err = generateNewIdentities(secret, 0, m.Spec.Replicas)
	if err != nil {
		return fmt.Errorf("could not place new identities in secret: %w", err)
	}
	secret.Data["CLUSTER_SECRET"] = []byte(clusterSecret)
	secret.Data["BOOTSTRAP_PEER_PRIV_KEY"] = []byte(bootstrapPrivateKey)
	secret.StringData["BOOTSTRAP_PEER_ID"] = peerID.String()

	// ensure reference is set
	if err = ctrl.SetControllerReference(m, secret, r.Scheme); err != nil {
		return err
	}
	err = r.Create(ctx, secret)
	return
}

// countIdentities Counts the amount of unique peer identities present in the secret.
func countIdentities(secret *corev1.Secret) int32 {
	var count int32
	for key := range secret.Data {
		if strings.Contains(key, peerIDPrefix) {
			count++
		}
	}
	return count
}

// generateNewIdentities Populates the secret data with new Peer IDs
// and private keys which are mapped based on the replica number.
func generateNewIdentities(secret *corev1.Secret, start, n int32) error {
	if secret.StringData == nil {
		secret.StringData = make(map[string]string, 0)
	}
	for i := start; i < n; i++ {
		// generate new private key & peer id
		peerID, privKey, err := utils.GenerateIdentity()
		if err != nil {
			return err
		}
		peerIDKey := peerIDPrefix + strconv.Itoa(int(i))
		secret.StringData[peerIDKey] = peerID.String()
		secretKey := privateKeyPrefix + strconv.Itoa(int(i))
		secret.StringData[secretKey] = privKey
	}
	return nil
}
