//go:build e2e
// +build e2e

package e2e

import (
	"fmt"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var projectImage = "ghcr.io/seamlezz/surrealdb-credential-operator:e2e"

func TestE2E(t *testing.T) {
	RegisterFailHandler(Fail)
	_, _ = fmt.Fprintf(GinkgoWriter, "Starting SurrealDB credential operator e2e suite\n")
	RunSpecs(t, "e2e suite")
}
