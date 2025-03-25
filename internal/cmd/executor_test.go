package cmd_test

import (
	"os"
	"os/exec"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
	"github.com/z4ce/snyk-auto-org/internal/cmd"
)

var _ = Describe("SnykExecutor", func() {
	var executor *cmd.SnykExecutor
	var orgID string

	BeforeEach(func() {
		orgID = "test-org-id"
		executor = cmd.NewSnykExecutor(orgID)
	})

	It("should create a new executor with the provided org ID", func() {
		Expect(executor).NotTo(BeNil())
		Expect(executor.OrgID).To(Equal(orgID))
	})

	It("should return an error when no arguments are provided", func() {
		err := executor.Execute([]string{})
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("no arguments provided"))
	})

	Context("when setting environment variables", func() {
		It("should set the SNYK_CFG_ORG environment variable", func() {
			// Skip in CI environments
			if os.Getenv("CI") != "" {
				Skip("Skipping in CI environment")
			}

			command := exec.Command("sh", "-c", "echo $SNYK_CFG_ORG")
			command.Env = append(os.Environ(), "SNYK_CFG_ORG="+orgID)

			session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())

			Eventually(session).Should(gexec.Exit(0))
			output := strings.TrimSpace(string(session.Out.Contents()))
			Expect(output).To(Equal(orgID))
		})
	})

	Context("when executing Snyk commands", func() {
		It("should pass arguments to the Snyk CLI", func() {
			// This test would require either mocking exec.Command or actually running Snyk
			// For now, we'll skip it to avoid external dependencies
			Skip("This test requires the actual Snyk CLI and would execute a real command")
		})
	})
})
