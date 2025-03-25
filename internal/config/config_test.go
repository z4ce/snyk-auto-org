package config_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spf13/viper"
	"github.com/z4ce/snyk-auto-org/internal/config"
)

var _ = Describe("Config", func() {
	var (
		tempDir   string
		configDir string
	)

	BeforeEach(func() {
		// Create a temporary directory for testing
		var err error
		tempDir, err = os.MkdirTemp("", "snyk-auto-org-config-test")
		Expect(err).NotTo(HaveOccurred())

		// Create a config directory in the temporary directory
		configDir = filepath.Join(tempDir, ".config", "snyk-auto-org")
		err = os.MkdirAll(configDir, 0755)
		Expect(err).NotTo(HaveOccurred())

		// Redirect HOME environment variable to use our test directory
		origUserHome := os.Getenv("HOME")
		DeferCleanup(func() {
			os.Setenv("HOME", origUserHome)
			viper.Reset()
		})
		os.Setenv("HOME", tempDir)
	})

	AfterEach(func() {
		os.RemoveAll(tempDir)
	})

	Describe("LoadConfig", func() {
		Context("when the config file does not exist", func() {
			It("should create a default config file and load default values", func() {
				// Load the config
				cfg, err := config.LoadConfig()
				Expect(err).NotTo(HaveOccurred())
				Expect(cfg).NotTo(BeNil())

				// Verify default values
				Expect(cfg.CacheTTL).To(Equal(24 * time.Hour))
				Expect(cfg.DefaultOrg).To(Equal(""))
				Expect(cfg.Verbose).To(BeFalse())

				// Verify the config file was created
				configFile := filepath.Join(configDir, "config.json")
				Expect(configFile).To(BeARegularFile())

				// Verify the file content
				data, err := os.ReadFile(configFile)
				Expect(err).NotTo(HaveOccurred())

				var fileContent map[string]interface{}
				err = json.Unmarshal(data, &fileContent)
				Expect(err).NotTo(HaveOccurred())
				Expect(fileContent["cache_ttl"]).To(Equal("24h"))
				Expect(fileContent["default_org"]).To(Equal(""))
				Expect(fileContent["verbose"]).To(Equal(false))
			})
		})

		Context("when the config file exists with custom values", func() {
			BeforeEach(func() {
				// Create a config file with custom values
				configFile := filepath.Join(configDir, "config.json")
				content := `{
					"cache_ttl": "1h",
					"default_org": "my-org",
					"verbose": true
				}`
				err := os.WriteFile(configFile, []byte(content), 0644)
				Expect(err).NotTo(HaveOccurred())
			})

			It("should load values from the file", func() {
				// Load the config
				cfg, err := config.LoadConfig()
				Expect(err).NotTo(HaveOccurred())
				Expect(cfg).NotTo(BeNil())

				// Verify custom values
				Expect(cfg.CacheTTL).To(Equal(1 * time.Hour))
				Expect(cfg.DefaultOrg).To(Equal("my-org"))
				Expect(cfg.Verbose).To(BeTrue())
			})
		})

		Context("when the config file contains an invalid cache TTL", func() {
			BeforeEach(func() {
				// Create a config file with an invalid TTL
				configFile := filepath.Join(configDir, "config.json")
				content := `{
					"cache_ttl": "invalid",
					"default_org": "",
					"verbose": false
				}`
				err := os.WriteFile(configFile, []byte(content), 0644)
				Expect(err).NotTo(HaveOccurred())
			})

			It("should return an error", func() {
				// Load the config
				cfg, err := config.LoadConfig()
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("invalid cache TTL"))
				Expect(cfg).To(BeNil())
			})
		})
	})

	Describe("SaveConfig", func() {
		It("should save the configuration to disk", func() {
			// Create a configuration
			cfg := &config.Config{
				CacheTTL:   2 * time.Hour,
				DefaultOrg: "test-org",
				Verbose:    true,
			}

			// We need to load first to initialize viper
			_, err := config.LoadConfig()
			Expect(err).NotTo(HaveOccurred())

			// Save the configuration
			err = config.SaveConfig(cfg)
			Expect(err).NotTo(HaveOccurred())

			// Verify the file content
			configFile := filepath.Join(configDir, "config.json")
			data, err := os.ReadFile(configFile)
			Expect(err).NotTo(HaveOccurred())

			var fileContent map[string]interface{}
			err = json.Unmarshal(data, &fileContent)
			Expect(err).NotTo(HaveOccurred())
			Expect(fileContent["cache_ttl"]).To(Equal("2h0m0s"))
			Expect(fileContent["default_org"]).To(Equal("test-org"))
			Expect(fileContent["verbose"]).To(Equal(true))

			// Load again to verify loaded values match saved values
			loadedCfg, err := config.LoadConfig()
			Expect(err).NotTo(HaveOccurred())
			Expect(loadedCfg.CacheTTL).To(Equal(2 * time.Hour))
			Expect(loadedCfg.DefaultOrg).To(Equal("test-org"))
			Expect(loadedCfg.Verbose).To(BeTrue())
		})
	})
})
