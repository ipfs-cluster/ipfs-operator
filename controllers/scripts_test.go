package controllers_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	v1 "k8s.io/api/core/v1"

	"github.com/redhat-et/ipfs-operator/api/v1alpha1"
	"github.com/redhat-et/ipfs-operator/controllers"
)

var _ = Describe("IPFS Reconciler", func() {
	var ipfsReconciler *controllers.IpfsReconciler
	var ipfs *v1alpha1.Ipfs
	var configMap *v1.ConfigMap
	var ctx context.Context

	BeforeEach(func() {
		ctx = context.TODO()
		ipfsReconciler = &controllers.IpfsReconciler{
			Scheme: k8sClient.Scheme(),
			Client: k8sClient,
		}
		configMap = &v1.ConfigMap{}
		ipfs = &v1alpha1.Ipfs{}
	})

	When("ConfigMapScripts are edited", func() {
		const myName = "my-fav-ipfs-node"
		BeforeEach(func() {
			// uses the name
			ipfs.Name = myName
		})

		It("populates the ConfigMap", func() {
			// configMap is empty
			Expect(len(configMap.Data)).To(Equal(0))
			fn, _ := ipfsReconciler.ConfigMapScripts(ctx, ipfs, configMap)
			// should not have errored
			Expect(fn()).NotTo(HaveOccurred())
			// the configmap should be populated with the following scripts
			Expect(len(configMap.Data)).To(Equal(2))

			expectedKeys := []string{
				controllers.ScriptConfigureIPFS,
				controllers.ScriptIPFSClusterEntryPoint,
			}
			for _, key := range expectedKeys {
				data, ok := configMap.Data[key]
				Expect(ok).To(BeTrue())
				Expect(data).NotTo(BeEmpty())
			}
		})

		It("contains the IPFS resource name", func() {
			fn, name := ipfsReconciler.ConfigMapScripts(ctx, ipfs, configMap)
			Expect(fn).NotTo(BeNil())
			Expect(fn()).NotTo(HaveOccurred())
			Expect(name).To(ContainSubstring(myName))
			Expect(name).To(Equal("ipfs-cluster-scripts-" + myName))
		})
	})
})
