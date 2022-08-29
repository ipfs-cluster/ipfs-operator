//go:build tools
// +build tools

package main

// this file exists so that we can store the dependencies from the
// ginkgo/v2 binary within our go.mod file
import (
	_ "github.com/onsi/ginkgo/v2/ginkgo"
)
