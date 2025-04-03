package api_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/z4ce/snyk-auto-org/internal/api"
)

// MockTokenProvider implements TokenProvider for testing
type MockTokenProvider struct {
	token *api.TokenStorage
	err   error
	saved *api.TokenStorage
}

func (m *MockTokenProvider) GetToken() (*api.TokenStorage, error) {
	return m.token, m.err
}

func (m *MockTokenProvider) SaveToken(token *api.TokenStorage) error {
	m.saved = token
	return nil
}

// MockTokenRefresher implements TokenRefresher for testing
type MockTokenRefresher struct {
	response *api.TokenResponse
	err      error
}

func (m *MockTokenRefresher) RefreshToken(refreshToken string) (*api.TokenResponse, error) {
	return m.response, m.err
}

var _ = Describe("SnykClient", func() {
	var (
		server   *httptest.Server
		client   *api.SnykClient
		mux      *http.ServeMux
		token    = "test-token"
		orgID    = "test-org-id"
		targetID = "test-target-id"
		gitURL   = "https://github.com/test/repo"
	)

	BeforeEach(func() {
		mux = http.NewServeMux()
		server = httptest.NewServer(mux)

		client = &api.SnykClient{
			APIToken:    token,
			RestBaseURL: server.URL,
			HTTPClient:  http.DefaultClient,
		}
	})

	AfterEach(func() {
		server.Close()
	})

	Describe("Token Refresh", func() {
		var (
			mockProvider  *MockTokenProvider
			mockRefresher *MockTokenRefresher
		)

		BeforeEach(func() {
			mockProvider = &MockTokenProvider{}
			mockRefresher = &MockTokenRefresher{}
		})

		Context("when the token needs to be refreshed", func() {
			BeforeEach(func() {
				// Set up expired token storage
				mockProvider.token = &api.TokenStorage{
					AccessToken:  "expired-token",
					TokenType:    "bearer",
					RefreshToken: "test-refresh-token",
					Expiry:       time.Now().Add(-1 * time.Hour), // Expired 1 hour ago
				}

				// Set up mock refresher response
				mockRefresher.response = &api.TokenResponse{
					AccessToken:      "new-test-token",
					ExpiresIn:        3600,
					RefreshToken:     "new-refresh-token",
					RefreshExpiresIn: 15552000,
					TokenType:        "bearer",
					Scope:            "org.read org.project.read",
					BotID:            "test-bot-id",
				}
			})

			It("should refresh the token when expired", func() {
				token, err := api.GetSnykAPIToken(mockProvider, mockRefresher)
				Expect(err).NotTo(HaveOccurred())
				Expect(token).To(Equal("new-test-token"))

				// Verify the new token was saved
				Expect(mockProvider.saved).NotTo(BeNil())
				Expect(mockProvider.saved.AccessToken).To(Equal("new-test-token"))
				Expect(mockProvider.saved.RefreshToken).To(Equal("new-refresh-token"))
				Expect(mockProvider.saved.TokenType).To(Equal("bearer"))
				// Verify expiry was set correctly (within 1 second tolerance)
				expectedExpiry := time.Now().Add(3600 * time.Second)
				Expect(mockProvider.saved.Expiry).To(BeTemporally("~", expectedExpiry, time.Second))
			})
		})

		Context("when the token refresh fails", func() {
			BeforeEach(func() {
				// Set up expired token storage
				mockProvider.token = &api.TokenStorage{
					AccessToken:  "expired-token",
					TokenType:    "bearer",
					RefreshToken: "test-refresh-token",
					Expiry:       time.Now().Add(-1 * time.Hour), // Expired 1 hour ago
				}

				// Set up mock refresher to return error
				mockRefresher.err = fmt.Errorf("failed to refresh token: invalid_grant")
			})

			It("should return an error", func() {
				_, err := api.GetSnykAPIToken(mockProvider, mockRefresher)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("failed to refresh token"))
			})
		})

		Context("when the token is valid and not near expiry", func() {
			BeforeEach(func() {
				// Set up valid token storage
				mockProvider.token = &api.TokenStorage{
					AccessToken:  "valid-token",
					TokenType:    "bearer",
					RefreshToken: "test-refresh-token",
					Expiry:       time.Now().Add(1 * time.Hour), // Valid for 1 more hour
				}
			})

			It("should return the existing token without refreshing", func() {
				token, err := api.GetSnykAPIToken(mockProvider, mockRefresher)
				Expect(err).NotTo(HaveOccurred())
				Expect(token).To(Equal("valid-token"))

				// Verify no token was saved (no refresh occurred)
				Expect(mockProvider.saved).To(BeNil())
			})
		})

		Context("when the token is expired and no refresh token is available", func() {
			BeforeEach(func() {
				// Set up expired token storage with no refresh token
				mockProvider.token = &api.TokenStorage{
					AccessToken:  "expired-token",
					TokenType:    "bearer",
					RefreshToken: "",
					Expiry:       time.Now().Add(-1 * time.Hour), // Expired 1 hour ago
				}
			})

			It("should return an error", func() {
				_, err := api.GetSnykAPIToken(mockProvider, mockRefresher)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("no refresh token available"))
			})
		})
	})

	Describe("GetOrganizations", func() {
		Context("when the API returns a successful response", func() {
			BeforeEach(func() {
				mux.HandleFunc("/orgs", func(w http.ResponseWriter, r *http.Request) {
					Expect(r.Header.Get("Authorization")).To(Equal("Bearer " + token))
					Expect(r.URL.Query().Get("version")).To(Equal(api.SnykAPIRestVersion))
					w.WriteHeader(http.StatusOK)
					w.Write([]byte(`{
						"data": [
							{
								"id": "org-id-1",
								"attributes": {
									"name": "Organization 1",
									"slug": "org-slug-1"
								}
							},
							{
								"id": "org-id-2",
								"attributes": {
									"name": "Organization 2",
									"slug": "org-slug-2"
								}
							}
						]
					}`))
				})
			})

			It("returns the list of organizations", func() {
				orgs, err := client.GetOrganizations()
				Expect(err).NotTo(HaveOccurred())
				Expect(orgs).To(HaveLen(2))
				Expect(orgs[0].ID).To(Equal("org-id-1"))
				Expect(orgs[0].Name).To(Equal("Organization 1"))
				Expect(orgs[0].Slug).To(Equal("org-slug-1"))
				Expect(orgs[1].ID).To(Equal("org-id-2"))
				Expect(orgs[1].Name).To(Equal("Organization 2"))
				Expect(orgs[1].Slug).To(Equal("org-slug-2"))
			})
		})

		Context("when the API returns paginated results", func() {
			BeforeEach(func() {
				// First page with next link
				mux.HandleFunc("/orgs", func(w http.ResponseWriter, r *http.Request) {
					Expect(r.Header.Get("Authorization")).To(Equal("Bearer " + token))
					Expect(r.URL.Query().Get("version")).To(Equal(api.SnykAPIRestVersion))

					// Check if this is the second page request
					if r.URL.Query().Get("starting_after") == "org-id-2" {
						w.WriteHeader(http.StatusOK)
						w.Write([]byte(`{
							"data": [
								{
									"id": "org-id-3",
									"attributes": {
										"name": "Organization 3",
										"slug": "org-slug-3"
									}
								},
								{
									"id": "org-id-4",
									"attributes": {
										"name": "Organization 4",
										"slug": "org-slug-4"
									}
								}
							],
							"links": {
								"prev": "/orgs?version=` + api.SnykAPIRestVersion + `&ending_before=org-id-3"
							}
						}`))
						return
					}

					// First page
					w.WriteHeader(http.StatusOK)
					w.Write([]byte(`{
						"data": [
							{
								"id": "org-id-1",
								"attributes": {
									"name": "Organization 1",
									"slug": "org-slug-1"
								}
							},
							{
								"id": "org-id-2",
								"attributes": {
									"name": "Organization 2",
									"slug": "org-slug-2"
								}
							}
						],
						"links": {
							"next": "/orgs?version=` + api.SnykAPIRestVersion + `&starting_after=org-id-2"
						}
					}`))
				})
			})

			It("returns all organizations from all pages", func() {
				orgs, err := client.GetOrganizations()
				Expect(err).NotTo(HaveOccurred())
				Expect(orgs).To(HaveLen(4))
				Expect(orgs[0].ID).To(Equal("org-id-1"))
				Expect(orgs[1].ID).To(Equal("org-id-2"))
				Expect(orgs[2].ID).To(Equal("org-id-3"))
				Expect(orgs[3].ID).To(Equal("org-id-4"))
			})
		})

		Context("when the API returns paginated results with unicode-encoded ampersands", func() {
			BeforeEach(func() {
				// First page with next link containing unicode-encoded ampersand
				mux.HandleFunc("/orgs", func(w http.ResponseWriter, r *http.Request) {
					Expect(r.Header.Get("Authorization")).To(Equal("Bearer " + token))
					Expect(r.URL.Query().Get("version")).To(Equal(api.SnykAPIRestVersion))

					// Check if this is the second page request
					if r.URL.Query().Get("starting_after") == "org-id-2" {
						w.WriteHeader(http.StatusOK)
						w.Write([]byte(`{
							"data": [
								{
									"id": "org-id-3",
									"attributes": {
										"name": "Organization 3",
										"slug": "org-slug-3"
									}
								}
							]
						}`))
						return
					}

					// First page with unicode-encoded ampersand
					w.WriteHeader(http.StatusOK)
					w.Write([]byte(`{
						"data": [
							{
								"id": "org-id-1",
								"attributes": {
									"name": "Organization 1",
									"slug": "org-slug-1"
								}
							},
							{
								"id": "org-id-2",
								"attributes": {
									"name": "Organization 2",
									"slug": "org-slug-2"
								}
							}
						],
						"links": {
							"next": "/orgs?version=2024-10-15\u0026starting_after=org-id-2"
						}
					}`))
				})
			})

			It("should handle unicode-encoded ampersands in pagination links", func() {
				orgs, err := client.GetOrganizations()
				Expect(err).NotTo(HaveOccurred())
				Expect(orgs).To(HaveLen(3))
				Expect(orgs[0].ID).To(Equal("org-id-1"))
				Expect(orgs[1].ID).To(Equal("org-id-2"))
				Expect(orgs[2].ID).To(Equal("org-id-3"))
			})
		})

		Context("when pagination links contain absolute URLs with base path", func() {
			BeforeEach(func() {
				mux.HandleFunc("/orgs", func(w http.ResponseWriter, r *http.Request) {
					Expect(r.Header.Get("Authorization")).To(Equal("Bearer " + token))
					Expect(r.URL.Query().Get("version")).To(Equal(api.SnykAPIRestVersion))

					// Check if this is the second page request
					if r.URL.Query().Get("starting_after") == "org-id-2" {
						w.WriteHeader(http.StatusOK)
						w.Write([]byte(`{
							"data": [
								{
									"id": "org-id-3",
									"attributes": {
										"name": "Organization 3",
										"slug": "org-slug-3"
									}
								}
							]
						}`))
						return
					}

					// First page with absolute URL in next link that includes the base path
					w.WriteHeader(http.StatusOK)
					w.Write([]byte(`{
						"data": [
							{
								"id": "org-id-1",
								"attributes": {
									"name": "Organization 1",
									"slug": "org-slug-1"
								}
							},
							{
								"id": "org-id-2",
								"attributes": {
									"name": "Organization 2",
									"slug": "org-slug-2"
								}
							}
						],
						"links": {
							"next": "` + server.URL + `/orgs?version=2024-10-15&starting_after=org-id-2"
						}
					}`))
				})
			})

			It("should handle absolute URLs with base path in pagination links", func() {
				orgs, err := client.GetOrganizations()
				Expect(err).NotTo(HaveOccurred())
				Expect(orgs).To(HaveLen(3))
				Expect(orgs[0].ID).To(Equal("org-id-1"))
				Expect(orgs[1].ID).To(Equal("org-id-2"))
				Expect(orgs[2].ID).To(Equal("org-id-3"))
			})
		})

		Context("when the API returns an error", func() {
			BeforeEach(func() {
				mux.HandleFunc("/orgs", func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusUnauthorized)
				})
			})

			It("returns an error", func() {
				_, err := client.GetOrganizations()
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("unexpected status code: 401"))
			})
		})
	})

	Describe("GetTargetsWithURL", func() {
		Context("when the API returns paginated results", func() {
			BeforeEach(func() {
				// First page with next link
				mux.HandleFunc("/orgs/"+orgID+"/targets", func(w http.ResponseWriter, r *http.Request) {
					Expect(r.Header.Get("Authorization")).To(Equal("Bearer " + token))
					Expect(r.URL.Query().Get("version")).To(Equal(api.SnykAPIRestVersion))

					// Check if this is the second page request
					if r.URL.Query().Get("starting_after") == "target-id-1" {
						w.WriteHeader(http.StatusOK)
						w.Write([]byte(`{
							"data": [
								{
									"id": "target-id-2",
									"attributes": {
										"displayName": "test/repo2",
										"url": "https://github.com/test/repo2"
									}
								}
							],
							"links": {
								"prev": "/orgs/` + orgID + `/targets?version=` + api.SnykAPIRestVersion + `&ending_before=target-id-2"
							}
						}`))
						return
					}

					// First page
					w.WriteHeader(http.StatusOK)
					w.Write([]byte(`{
						"data": [
							{
								"id": "target-id-1",
								"attributes": {
									"displayName": "test/repo",
									"url": "` + gitURL + `"
								}
							}
						],
						"links": {
							"next": "/orgs/` + orgID + `/targets?version=` + api.SnykAPIRestVersion + `&starting_after=target-id-1"
						}
					}`))
				})
			})

			It("returns all targets from all pages", func() {
				targets, err := client.GetTargetsWithURL(orgID, "")
				Expect(err).NotTo(HaveOccurred())
				Expect(targets).To(HaveLen(2))
				Expect(targets[0].ID).To(Equal("target-id-1"))
				Expect(targets[0].Attributes.DisplayName).To(Equal("test/repo"))
				Expect(targets[1].ID).To(Equal("target-id-2"))
				Expect(targets[1].Attributes.DisplayName).To(Equal("test/repo2"))
			})
		})

		Context("when the API returns a successful response", func() {
			BeforeEach(func() {
				mux.HandleFunc("/orgs/"+orgID+"/targets", func(w http.ResponseWriter, r *http.Request) {
					Expect(r.Header.Get("Authorization")).To(Equal("Bearer " + token))
					Expect(r.URL.Query().Get("version")).To(Equal(api.SnykAPIRestVersion))
					Expect(r.URL.Query().Get("url")).To(Equal(gitURL))
					w.WriteHeader(http.StatusOK)
					w.Write([]byte(`{
						"data": [
							{
								"id": "` + targetID + `",
								"attributes": {
									"displayName": "test/repo",
									"url": "` + gitURL + `"
								}
							}
						]
					}`))
				})
			})

			It("returns the list of targets", func() {
				targets, err := client.GetTargetsWithURL(orgID, gitURL)
				Expect(err).NotTo(HaveOccurred())
				Expect(targets).To(HaveLen(1))
				Expect(targets[0].ID).To(Equal(targetID))
				Expect(targets[0].Attributes.DisplayName).To(Equal("test/repo"))
				Expect(targets[0].Attributes.URL).To(Equal(gitURL))
			})
		})

		Context("when pagination links contain absolute URLs with base path", func() {
			BeforeEach(func() {
				mux.HandleFunc("/orgs/"+orgID+"/targets", func(w http.ResponseWriter, r *http.Request) {
					Expect(r.Header.Get("Authorization")).To(Equal("Bearer " + token))
					Expect(r.URL.Query().Get("version")).To(Equal(api.SnykAPIRestVersion))

					// Check if this is the second page request
					if r.URL.Query().Get("starting_after") == "target-id-1" {
						w.WriteHeader(http.StatusOK)
						w.Write([]byte(`{
							"data": [
								{
									"id": "target-id-2",
									"attributes": {
										"displayName": "test/repo2",
										"url": "https://github.com/test/repo2"
									}
								}
							]
						}`))
						return
					}

					// First page with absolute URL in next link
					w.WriteHeader(http.StatusOK)
					w.Write([]byte(`{
						"data": [
							{
								"id": "target-id-1",
								"attributes": {
									"displayName": "test/repo",
									"url": "` + gitURL + `"
								}
							}
						],
						"links": {
							"next": "` + server.URL + `/orgs/` + orgID + `/targets?version=` + api.SnykAPIRestVersion + `&starting_after=target-id-1"
						}
					}`))
				})
			})

			It("should handle absolute URLs with base path in pagination links", func() {
				targets, err := client.GetTargetsWithURL(orgID, "")
				Expect(err).NotTo(HaveOccurred())
				Expect(targets).To(HaveLen(2))
				Expect(targets[0].ID).To(Equal("target-id-1"))
				Expect(targets[0].Attributes.DisplayName).To(Equal("test/repo"))
				Expect(targets[1].ID).To(Equal("target-id-2"))
				Expect(targets[1].Attributes.DisplayName).To(Equal("test/repo2"))
			})
		})

		Context("when the API returns an error", func() {
			BeforeEach(func() {
				mux.HandleFunc("/orgs/"+orgID+"/targets", func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusUnauthorized)
				})
			})

			It("returns an error", func() {
				_, err := client.GetTargetsWithURL(orgID, "")
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("unexpected status code: 401"))
			})
		})
	})

	Describe("FindOrgWithTargetURL", func() {
		Context("when an organization with the target URL exists", func() {
			BeforeEach(func() {
				// Mock the GetOrganizations response
				mux.HandleFunc("/orgs", func(w http.ResponseWriter, r *http.Request) {
					Expect(r.Header.Get("Authorization")).To(Equal("Bearer " + token))
					Expect(r.URL.Query().Get("version")).To(Equal(api.SnykAPIRestVersion))
					w.WriteHeader(http.StatusOK)
					w.Write([]byte(`{
						"data": [
							{
								"id": "org-id-1",
								"attributes": {
									"name": "Organization 1",
									"slug": "org-slug-1"
								}
							},
							{
								"id": "org-id-2",
								"attributes": {
									"name": "Organization 2",
									"slug": "org-slug-2"
								}
							}
						]
					}`))
				})

				// Mock the GetTargetsWithURL response for the first org (no targets)
				mux.HandleFunc("/orgs/org-id-1/targets", func(w http.ResponseWriter, r *http.Request) {
					Expect(r.Header.Get("Authorization")).To(Equal("Bearer " + token))
					w.WriteHeader(http.StatusOK)
					w.Write([]byte(`{"data": []}`))
				})

				// Mock the GetTargetsWithURL response for the second org (has target)
				mux.HandleFunc("/orgs/org-id-2/targets", func(w http.ResponseWriter, r *http.Request) {
					Expect(r.Header.Get("Authorization")).To(Equal("Bearer " + token))
					w.WriteHeader(http.StatusOK)
					w.Write([]byte(`{
						"data": [
							{
								"id": "` + targetID + `",
								"attributes": {
									"displayName": "test/repo",
									"url": "` + gitURL + `"
								}
							}
						]
					}`))
				})
			})

			It("returns the organization with target URL", func() {
				orgTarget, err := client.FindOrgWithTargetURL(gitURL)
				Expect(err).NotTo(HaveOccurred())
				Expect(orgTarget).NotTo(BeNil())
				Expect(orgTarget.OrgID).To(Equal("org-id-2"))
				Expect(orgTarget.OrgName).To(Equal("Organization 2"))
				Expect(orgTarget.TargetURL).To(Equal(gitURL))
				Expect(orgTarget.TargetName).To(Equal("test/repo"))
			})
		})

		Context("when no organization has the target URL", func() {
			BeforeEach(func() {
				// Mock the GetOrganizations response
				mux.HandleFunc("/orgs", func(w http.ResponseWriter, r *http.Request) {
					Expect(r.Header.Get("Authorization")).To(Equal("Bearer " + token))
					Expect(r.URL.Query().Get("version")).To(Equal(api.SnykAPIRestVersion))
					w.WriteHeader(http.StatusOK)
					w.Write([]byte(`{
						"data": [
							{
								"id": "org-id-1",
								"attributes": {
									"name": "Organization 1",
									"slug": "org-slug-1"
								}
							}
						]
					}`))
				})

				// Mock the GetTargetsWithURL response (no targets)
				mux.HandleFunc("/orgs/org-id-1/targets", func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusOK)
					w.Write([]byte(`{"data": []}`))
				})
			})

			It("returns an error", func() {
				_, err := client.FindOrgWithTargetURL(gitURL)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("no organization found with a target matching URL"))
			})
		})
	})
})
