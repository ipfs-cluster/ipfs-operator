package controllers_test

import (
	"context"
	"encoding/base64"
	"math/rand"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/redhat-et/ipfs-operator/api/v1alpha1"
	"github.com/redhat-et/ipfs-operator/controllers"
)

var _ = Describe("IPFS Reconciler", func() {
	var ipfsReconciler *controllers.IpfsClusterReconciler
	var ipfs *v1alpha1.IpfsCluster
	var configMapScripts *v1.ConfigMap
	var ctx context.Context
	const myName = "my-fav-ipfs-node"

	BeforeEach(func() {
		ctx = context.TODO()
		ipfsReconciler = &controllers.IpfsClusterReconciler{
			Scheme: k8sClient.Scheme(),
			Client: k8sClient,
		}
		configMapScripts = &v1.ConfigMap{}
		ipfs = &v1alpha1.IpfsCluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      myName,
				Namespace: "test",
			},
		}
	})

	When("ConfigMapScripts are edited", func() {
		It("populates the ConfigMap", func() {
			// configMap is empty
			Expect(len(configMapScripts.Data)).To(Equal(0))
			fn, _ := ipfsReconciler.ConfigMapScripts(ctx, ipfs, configMapScripts)
			// should not have errored
			Expect(fn()).NotTo(HaveOccurred())
			// the configmap should be populated with the following scripts
			Expect(len(configMapScripts.Data)).To(Equal(2))

			expectedKeys := []string{
				controllers.ScriptConfigureIPFS,
				controllers.ScriptIPFSClusterEntryPoint,
			}
			for _, key := range expectedKeys {
				data, ok := configMapScripts.Data[key]
				Expect(ok).To(BeTrue())
				Expect(data).NotTo(BeEmpty())
			}
		})

		It("contains the IPFS resource name", func() {
			fn, name := ipfsReconciler.ConfigMapScripts(ctx, ipfs, configMapScripts)
			Expect(fn).NotTo(BeNil())
			Expect(fn()).NotTo(HaveOccurred())
			Expect(name).To(ContainSubstring(myName))
			Expect(name).To(Equal("ipfs-cluster-scripts-" + myName))
		})
	})

	When("replicas are edited", func() {
		// we always expect there to be cluster secrets, which have two values
		const alwaysKeys = 2
		var (
			clusterSec   = []byte("cluster secret")
			bootstrapKey = []byte("bootstrap private key")
			replicas     int32
		)
		BeforeEach(func() {
			replicas = rand.Int31n(100)
			ipfs.Spec.Replicas = replicas
		})
		It("creates a new peer ids", func() {
			secretConfig := &v1.Secret{}
			fn, _ := ipfsReconciler.SecretConfig(ctx, ipfs, secretConfig, clusterSec, bootstrapKey)
			Expect(fn()).To(BeNil())
			secretStringToData(secretConfig)
			expectedKeys := int(replicas)*2 + alwaysKeys
			Expect(len(secretConfig.Data)).To(Equal(expectedKeys))

			// increase the replica count. Expect to see new keys generated.
			ipfs.Spec.Replicas++
			fn, _ = ipfsReconciler.SecretConfig(ctx, ipfs, secretConfig, clusterSec, bootstrapKey)
			Expect(fn()).To(BeNil())
			secretStringToData(secretConfig)
			Expect(len(secretConfig.Data)).To(Equal(expectedKeys + 2))

		})
	})
})

// The k8s client will encode and copy data from the StringData to Data
// This function mimics the behavior for tests.
func secretStringToData(secret *v1.Secret) {
	for k, v := range secret.StringData {
		bv := []byte(v)
		enc := make([]byte, base64.StdEncoding.EncodedLen(len(bv)))
		base64.StdEncoding.Encode(enc, bv)
		secret.Data[k] = enc
	}
	secret.StringData = make(map[string]string)
}
