package api_test

import (
	"net/http"
	"net/http/httptest"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/z4ce/snyk-auto-org/internal/api"
)

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
					Expect(r.URL.Query().Get("url")).To(Equal(gitURL))
					w.WriteHeader(http.StatusOK)
					w.Write([]byte(`{"data": []}`))
				})

				// Mock the GetTargetsWithURL response for the second org (has target)
				mux.HandleFunc("/orgs/org-id-2/targets", func(w http.ResponseWriter, r *http.Request) {
					Expect(r.Header.Get("Authorization")).To(Equal("Bearer " + token))
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
