package controllers_test

import (
	"context"
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
	var secretConfig *v1.Secret
	var ctx context.Context
	const myName = "my-fav-ipfs-node"

	BeforeEach(func() {
		ctx = context.TODO()
		ipfsReconciler = &controllers.IpfsClusterReconciler{
			Scheme: k8sClient.Scheme(),
			Client: k8sClient,
		}
		configMapScripts = &v1.ConfigMap{}
		secretConfig = &v1.Secret{}
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
			replicas     = rand.Int31n(100)
			clusterSec   = []byte("cluster secret")
			bootstrapKey = []byte("bootstrap private key")
		)
		BeforeEach(func() {
			ipfs.Spec.Replicas = replicas
		})
		It("creates a new peer ids", func() {
			fn, _ := ipfsReconciler.SecretConfig(ctx, ipfs, secretConfig, clusterSec, bootstrapKey)
			Expect(fn()).To(BeNil())
			expectedKeys := replicas * 2
			// in the real world, StringData will be copied to Data as part of k8s.
			// However, we are checking it before it has the opportunity.
			Expect(len(secretConfig.Data)).To(Equal(alwaysKeys))
			Expect(len(secretConfig.StringData)).To(Equal(expectedKeys))

			// save this so we can check for changes later.
			dataCopy := make(map[string][]byte)
			stringCopy := make(map[string]string)
			for k, v := range secretConfig.Data {
				dataCopy[k] = v
			}
			for k, v := range secretConfig.StringData {
				stringCopy[k] = v
			}

			// increase the replica count. Expect to see new keys generated.
			ipfs.Spec.Replicas++
			fn, _ = ipfsReconciler.SecretConfig(ctx, ipfs, secretConfig, clusterSec, bootstrapKey)
			Expect(fn()).To(BeNil())
			Expect(len(secretConfig.Data)).To(Equal(alwaysKeys))
			Expect(len(secretConfig.StringData)).To(Equal(expectedKeys + 2))

			// expect the old keys to still be the same
			for k := range dataCopy {
				Expect(dataCopy[k]).To(Equal(secretConfig.Data[k]))
			}
			for k := range stringCopy {
				Expect(stringCopy[k]).To(Equal(secretConfig.StringData[k]))
			}
		})
	})
})
