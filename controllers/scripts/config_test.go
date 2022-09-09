package scripts_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/redhat-et/ipfs-operator/controllers/scripts"
)

var _ = Describe("Bloom Filter", func() {

	When("storage values are provided in decimal SI", func() {
		It("returns the correct value", func() {
			testCases := []struct {
				Input         int64
				ExpectedBytes int64
			}{
				{
					// N = 2
					Input:         int64(2 * scripts.BloomBlockSize),
					ExpectedBytes: 4,
				},
				{
					// N = 1, same size as block
					Input:         int64(1 * scripts.BloomBlockSize),
					ExpectedBytes: 2,
				},
				{
					// N = 57
					Input:         int64(57 * scripts.BloomBlockSize),
					ExpectedBytes: 107,
				},
				{
					// N = 0, invalid value
					Input:         int64(0),
					ExpectedBytes: 0,
				},
				{
					// N != 0, but N < 1
					Input:         int64(1),
					ExpectedBytes: 0,
				},
				{
					// Size just 1 Byte smaller than N = 1
					Input:         int64(1*scripts.BloomBlockSize - 1),
					ExpectedBytes: 0,
				},
			}
			for _, test := range testCases {
				output := scripts.CalculateBloomFilterSize(test.Input)
				Expect(output).To(Equal(test.ExpectedBytes))
			}
		})
	})
})
