package cache_test

import (
	"os"
	"path/filepath"
	"time"

	_ "github.com/mattn/go-sqlite3"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/z4ce/snyk-auto-org/internal/api"
	"github.com/z4ce/snyk-auto-org/internal/cache"
)

var _ = Describe("SQLiteCache", func() {
	var (
		tempDir       string
		cacheDir      string
		dbCache       *cache.SQLiteCache
		organizations []api.Organization
	)

	BeforeEach(func() {
		// Create a temporary directory for testing
		var err error
		tempDir, err = os.MkdirTemp("", "snyk-auto-org-cache-test")
		Expect(err).NotTo(HaveOccurred())

		// Create a cache directory in the temporary directory
		cacheDir = filepath.Join(tempDir, ".config", "snyk-auto-org")
		err = os.MkdirAll(cacheDir, 0755)
		Expect(err).NotTo(HaveOccurred())

		// Redirect HOME environment variable to use our test directory
		origUserHome := os.Getenv("HOME")
		DeferCleanup(func() {
			os.Setenv("HOME", origUserHome)
		})
		os.Setenv("HOME", tempDir)

		// Sample test organizations
		organizations = []api.Organization{
			{
				ID:   "org-id-1",
				Name: "Organization 1",
				Slug: "org-1",
			},
			{
				ID:   "org-id-2",
				Name: "Organization 2",
				Slug: "org-2",
			},
		}

		// Create a new SQLite cache
		dbCache, err = cache.NewSQLiteCache()
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		if dbCache != nil {
			Expect(dbCache.Close()).To(Succeed())
		}
		os.RemoveAll(tempDir)
	})

	Describe("StoreOrganizations and GetOrganizations", func() {
		It("should store and retrieve organizations", func() {
			// Store the test organizations
			err := dbCache.StoreOrganizations(organizations)
			Expect(err).NotTo(HaveOccurred())

			// Retrieve the organizations
			retrievedOrgs, err := dbCache.GetOrganizations()
			Expect(err).NotTo(HaveOccurred())
			Expect(retrievedOrgs).To(HaveLen(2))
			Expect(retrievedOrgs[0].ID).To(Equal("org-id-1"))
			Expect(retrievedOrgs[0].Name).To(Equal("Organization 1"))
			Expect(retrievedOrgs[1].ID).To(Equal("org-id-2"))
			Expect(retrievedOrgs[1].Name).To(Equal("Organization 2"))
		})
	})

	Describe("IsExpired", func() {
		Context("when the cache is empty", func() {
			It("should report as expired", func() {
				expired, err := dbCache.IsExpired(24 * time.Hour)
				Expect(err).NotTo(HaveOccurred())
				Expect(expired).To(BeTrue())
			})
		})

		Context("when the cache is fresh", func() {
			BeforeEach(func() {
				err := dbCache.StoreOrganizations(organizations)
				Expect(err).NotTo(HaveOccurred())
			})

			It("should not report as expired", func() {
				expired, err := dbCache.IsExpired(24 * time.Hour)
				Expect(err).NotTo(HaveOccurred())
				Expect(expired).To(BeFalse())
			})
		})

		Context("when the cache is older than TTL", func() {
			It("should report as expired", func() {
				// This test is conceptual, as we can't easily manipulate time
				// In a real test, we might use a clock interface for better testability
				Skip("Testing time-based expiration requires refactoring for better testability")
			})
		})
	})

	Describe("ResetCache", func() {
		BeforeEach(func() {
			// Store some data first
			err := dbCache.StoreOrganizations(organizations)
			Expect(err).NotTo(HaveOccurred())

			// Verify data is stored
			storedOrgs, err := dbCache.GetOrganizations()
			Expect(err).NotTo(HaveOccurred())
			Expect(storedOrgs).To(HaveLen(2))
		})

		It("should clear all cached data", func() {
			// Reset the cache
			err := dbCache.ResetCache()
			Expect(err).NotTo(HaveOccurred())

			// Verify data is cleared
			orgs, err := dbCache.GetOrganizations()
			Expect(err).NotTo(HaveOccurred())
			Expect(orgs).To(BeEmpty())
		})
	})
})
