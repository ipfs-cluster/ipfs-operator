package utils

import "math"

// CalculateBloomFilterSize Accepts the size of the IPFS storage in bytes
// and returns the bloom filter size to be used in bytes.
//
// Computations are done with values based on this issue:
// https://github.com/redhat-et/ipfs-operator/issues/35#issue-1320941289
func CalculateBloomFilterSize(ipfsStorage int64) int64 {
	// 256KiB
	const blockSize = 262144
	// number of blocks
	var n, m int64
	// false-negative rate, 1 / 1000
	var p float64 = 0.001

	n = ipfsStorage / blockSize

	// formula based on bloom filter calculator:
	// https://hur.st/bloomfilter
	m = int64(math.Ceil((float64(n) * math.Log(p)) / math.Log(1/math.Pow(2, math.Log(2)))))
	return m
}
