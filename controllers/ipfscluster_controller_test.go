package controllers_test

import (
	"context"
	"encoding/base64"
	"math/rand"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
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
		const (
			clusterPeerID = "meow meow meow"
			alwaysKeys    = 3
		)
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
			fn, _ := ipfsReconciler.SecretConfig(ctx, ipfs, secretConfig, clusterSec, bootstrapKey, []byte(clusterPeerID))
			Expect(fn()).To(BeNil())
			secretStringToData(secretConfig)
			expectedKeys := int(replicas)*2 + alwaysKeys
			Expect(len(secretConfig.Data)).To(Equal(expectedKeys))

			// increase the replica count. Expect to see new keys generated.
			ipfs.Spec.Replicas++
			fn, _ = ipfsReconciler.SecretConfig(ctx, ipfs, secretConfig, clusterSec, bootstrapKey, []byte(clusterPeerID))
			Expect(fn()).To(BeNil())
			secretStringToData(secretConfig)
			Expect(len(secretConfig.Data)).To(Equal(expectedKeys + 2))

		})
	})
})

var _ = Describe("StatefulSet creation", func() {
	var ipfsReconciler *controllers.IpfsClusterReconciler
	var ipfs *v1alpha1.IpfsCluster
	var sts *appsv1.StatefulSet
	// var configMapScripts *v1.ConfigMap
	// var secret *v1.Secret
	// var svc *v1.Service
	// var ctx context.Context

	const (
		myName      = "my-fav-ipfs-node"
		scriptsName = "my-scripts"
		secretName  = "my-secret"
		svcName     = "my-svc"
		namespace   = "test"
	)
	BeforeEach(func() {
		// ctx = context.TODO()
		ipfsReconciler = &controllers.IpfsClusterReconciler{
			Scheme: k8sClient.Scheme(),
			Client: k8sClient,
		}
		// configMapScripts = &v1.ConfigMap{ObjectMeta: metav1.ObjectMeta{
		// 	Name: scriptsName,
		// }}
		ipfs = &v1alpha1.IpfsCluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      myName,
				Namespace: namespace,
			},
		}
		sts = &appsv1.StatefulSet{}
		// secret = &v1.Secret{ObjectMeta: metav1.ObjectMeta{Name: secretName}}
		// svc = &v1.Service{ObjectMeta: metav1.ObjectMeta{Name: svcName}}
	})

	When("ipfsResources is specified", func() {
		var cpuLimit = resource.NewQuantity(58, "m")
		var memoryLimit = resource.NewQuantity(10000, "m")
		var cpu = resource.NewQuantity(20, "m")
		var memory = resource.NewQuantity(40, "M")
		BeforeEach(func() {
			ipfs.Spec.IPFSResources = &v1.ResourceRequirements{
				Limits: v1.ResourceList{
					v1.ResourceLimitsCPU:    *cpuLimit,
					v1.ResourceLimitsMemory: *memoryLimit,
				},
				Requests: v1.ResourceList{
					v1.ResourceCPU:    *cpu,
					v1.ResourceMemory: *memory,
				},
			}
			ipfs.Spec.ClusterStorage = *resource.NewQuantity(1, "Gi")
			ipfs.Spec.IpfsStorage = *resource.NewQuantity(1, "Gi")
		})
		It("uses the IPFSCluster's IPFSResources setting", func() {
			mutFn := ipfsReconciler.StatefulSet(ipfs, sts, svcName, secretName, scriptsName)
			err := mutFn()
			Expect(err).NotTo(HaveOccurred())

			// find IPFS Container
			var ipfsContainer v1.Container
			for _, container := range sts.Spec.Template.Spec.Containers {
				if container.Name == controllers.ContainerIPFS {
					ipfsContainer = container
					break
				}
			}
			Expect(ipfsContainer).NotTo(BeNil())
			ipfsResources := ipfsContainer.Resources
			Expect(ipfsResources.Limits).NotTo(BeEmpty())
			Expect(ipfsResources.Limits[v1.ResourceLimitsCPU]).To(Equal(*cpuLimit))
			Expect(ipfsResources.Limits[v1.ResourceLimitsMemory]).To(Equal(*memoryLimit))
			Expect(ipfsResources.Requests).NotTo(BeEmpty())
			Expect(ipfsResources.Requests[v1.ResourceCPU]).To(Equal(*cpu))
			Expect(ipfsResources.Requests[v1.ResourceMemory]).To(Equal(*memory))
		})
	})

	When("ipfsResources is omitted", func() {
		BeforeEach(func() {
			ipfs.Spec.ClusterStorage = *resource.NewQuantity(5, "T")
			ipfs.Spec.IpfsStorage = *resource.NewQuantity(16, "T")
		})
		It("automatically computes resources requirements", func() {
			mutFn := ipfsReconciler.StatefulSet(ipfs, sts, svcName, secretName, scriptsName)
			err := mutFn()
			Expect(err).NotTo(HaveOccurred())

			// find IPFS Container
			var ipfsContainer v1.Container
			for _, container := range sts.Spec.Template.Spec.Containers {
				if container.Name == controllers.ContainerIPFS {
					ipfsContainer = container
					break
				}
			}
			Expect(ipfsContainer).NotTo(BeNil())
			ipfsResources := ipfsContainer.Resources
			Expect(ipfsResources.Limits).NotTo(BeEmpty())
			Expect(ipfsResources.Requests).NotTo(BeEmpty())

			cpuLimit := ipfsResources.Limits[v1.ResourceLimitsCPU]
			memoryLimit := ipfsResources.Limits[v1.ResourceLimitsMemory]
			Expect(cpuLimit.Value()).NotTo(Equal(0))
			Expect(memoryLimit.Value()).NotTo(Equal(0))

			cpu := ipfsResources.Requests[v1.ResourceCPU]
			memory := ipfsResources.Requests[v1.ResourceMemory]
			Expect(cpu.Value()).NotTo(Equal(0))
			Expect(memory.Value()).NotTo(Equal(0))
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
