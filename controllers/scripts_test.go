package controllers_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/redhat-et/ipfs-operator/api/v1alpha1"
	"github.com/redhat-et/ipfs-operator/controllers"
)

var _ = Describe("Scripts", func() {
	var ipfsReconciler *controllers.IpfsReconciler
	var ipfs *v1alpha1.Ipfs
	var ipfsConfigMap *v1.ConfigMap
	var ctx context.Context
	BeforeEach(func() {
		ctx = context.TODO()
		ipfsReconciler = &controllers.IpfsReconciler{
			Scheme: &runtime.Scheme{},
		}
		ipfsConfigMap = &v1.ConfigMap{}
		ipfs = &v1alpha1.Ipfs{}
	})
	It("works", func() {
		fn, _ := ipfsReconciler.ConfigMapScripts(ctx, ipfs, ipfsConfigMap)
		Expect(fn).NotTo(BeNil())
		Expect(fn()).To(HaveOccurred())

	})
})
