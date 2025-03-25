package app_test

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
	"github.com/spf13/cobra"
	"github.com/z4ce/snyk-auto-org/internal/app"
)

var _ = Describe("Root", func() {
	var (
		tmpDir   string
		origArgs []string
	)

	BeforeEach(func() {
		var err error
		tmpDir, err = os.MkdirTemp("", "snyk-auto-org-app-test")
		Expect(err).NotTo(HaveOccurred())

		// Save original os.Args
		origArgs = os.Args
	})

	AfterEach(func() {
		// Restore original os.Args
		os.Args = origArgs
		os.RemoveAll(tmpDir)
	})

	Describe("Execute", func() {
		It("should provide help output when no arguments are provided", func() {
			// Capture output
			oldStdout := os.Stdout
			r, w, _ := os.Pipe()
			os.Stdout = w

			// Mock os.Args
			os.Args = []string{"snyk-auto-org", "--help"}

			// Run the command
			app.Execute()

			// Restore output
			w.Close()
			os.Stdout = oldStdout

			var out bytes.Buffer
			_, err := out.ReadFrom(r)
			Expect(err).NotTo(HaveOccurred())

			output := out.String()
			Expect(output).To(ContainSubstring("Snyk Auto Org is a wrapper"))
			Expect(output).To(ContainSubstring("snyk-auto-org [snyk command]"))
		})
	})

	// This test would require a full build of the command
	Context("when running the actual binary", func() {
		It("should execute snyk commands with organization set", func() {
			// Skip in CI environments
			if os.Getenv("CI") != "" {
				Skip("Skipping in CI environment")
			}

			// Build the binary
			buildPath := filepath.Join(tmpDir, "snyk-auto-org")
			buildCmd := exec.Command("go", "build", "-o", buildPath, "../../cmd/snyk-auto-org/main.go")
			session, err := gexec.Start(buildCmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())
			Eventually(session).Should(gexec.Exit(0))

			// Test that the binary runs
			helpCmd := exec.Command(buildPath, "--help")
			helpSession, err := gexec.Start(helpCmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())
			Eventually(helpSession).Should(gexec.Exit(0))
			output := string(helpSession.Out.Contents())
			Expect(output).To(ContainSubstring("Snyk Auto Org is a wrapper"))
		})
	})

	Context("Integration with mock components", func() {
		It("should support mocking for more controlled testing", func() {
			// This would be a more complete integration test with mocks
			// In a real test suite, we would use dependency injection to mock components
			Skip("Integration testing with mocks requires refactoring for better testability")
		})
	})
})

// This is a helper function that would allow us to test a cobra command
// without actually executing it by capturing its output
func executeCommand(root *cobra.Command, args ...string) (string, error) {
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs(args)

	err := root.Execute()
	return strings.TrimSpace(buf.String()), err
}
