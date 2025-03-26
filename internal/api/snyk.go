package api

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os/exec"
	"strings"
	"time"
)

const (
	SnykAPIRestBaseURL = "https://api.snyk.io/rest"
	SnykConfigPath     = ".config/configstore/snyk.json"
	SnykAPIRestVersion = "2024-10-15"
	DefaultPageLimit   = 100 // Default number of items per page
)

// Organization represents a Snyk organization from the REST API
type Organization struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Slug       string `json:"slug"`
	Attributes struct {
		Name string `json:"name"`
		Slug string `json:"slug"`
	} `json:"attributes"`
}

// OrgsResponse represents the response from the Snyk REST API for organizations
type OrgsResponse struct {
	Data  []Organization `json:"data"`
	Links struct {
		Next string `json:"next"`
		Prev string `json:"prev"`
	} `json:"links"`
}

// Target represents a Snyk target from the REST API
type Target struct {
	ID         string `json:"id"`
	Attributes struct {
		DisplayName string `json:"displayName"`
		URL         string `json:"url"`
	} `json:"attributes"`
}

// TargetsResponse represents the response from the Snyk REST API for targets
type TargetsResponse struct {
	Data  []Target `json:"data"`
	Links struct {
		Next string `json:"next"`
		Prev string `json:"prev"`
	} `json:"links"`
}

// OrgTarget represents a combination of an organization and a target
type OrgTarget struct {
	OrgID      string
	OrgName    string
	TargetURL  string
	TargetName string
}

// SnykClient handles communication with the Snyk API
type SnykClient struct {
	APIToken    string
	RestBaseURL string
	HTTPClient  *http.Client
	PageLimit   int // Number of items per page for paginated requests
}

// NewSnykClient creates a new Snyk API client
func NewSnykClient() (*SnykClient, error) {
	token, err := getSnykAPIToken()
	if err != nil {
		return nil, err
	}

	return &SnykClient{
		APIToken:    token,
		RestBaseURL: SnykAPIRestBaseURL,
		HTTPClient: &http.Client{
			Timeout: 10 * time.Second,
		},
		PageLimit: DefaultPageLimit,
	}, nil
}

// redactToken returns a partially redacted version of the auth token
func redactToken(token string) string {
	if len(token) <= 8 {
		return "****"
	}
	return token[:4] + "..." + token[len(token)-4:]
}

// logRequest logs information about the API request being made
func (c *SnykClient) logRequest(method, url string) {
	redactedToken := redactToken(c.APIToken)
	log.Printf("Snyk API Request: %s %s [Auth: Bearer %s]", method, url, redactedToken)
}

// GetOrganizations retrieves the list of organizations from the Snyk REST API
func (c *SnykClient) GetOrganizations() ([]Organization, error) {
	params := url.Values{}
	params.Add("version", SnykAPIRestVersion)
	params.Add("limit", fmt.Sprintf("%d", c.PageLimit))

	reqURL := fmt.Sprintf("%s/orgs?%s", c.RestBaseURL, params.Encode())

	// Call the helper function to fetch all paginated results
	orgs, err := c.getAllOrganizationPages(reqURL)
	if err != nil {
		return nil, err
	}

	return orgs, nil
}

// getAllOrganizationPages retrieves all pages of organizations from the Snyk REST API
func (c *SnykClient) getAllOrganizationPages(initialURL string) ([]Organization, error) {
	var allOrganizations []Organization
	nextURL := initialURL

	for nextURL != "" {
		// Log the request
		c.logRequest("GET", nextURL)

		// Make request to the current URL
		req, err := http.NewRequest("GET", nextURL, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to create request: %w", err)
		}

		req.Header.Set("Content-Type", "application/vnd.api+json")
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.APIToken))

		resp, err := c.HTTPClient.Do(req)
		if err != nil {
			return nil, fmt.Errorf("failed to execute request: %w", err)
		}

		if resp.StatusCode != http.StatusOK {
			bodyBytes, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			return nil, fmt.Errorf("unexpected status code: %d, body: %s", resp.StatusCode, string(bodyBytes))
		}

		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			return nil, fmt.Errorf("failed to read response body: %w", err)
		}

		var orgsResp OrgsResponse
		if err := json.Unmarshal(body, &orgsResp); err != nil {
			return nil, fmt.Errorf("failed to unmarshal response: %w", err)
		}

		// Map API response to Organization objects and append to result
		for _, org := range orgsResp.Data {
			allOrganizations = append(allOrganizations, Organization{
				ID:   org.ID,
				Name: org.Attributes.Name,
				Slug: org.Attributes.Slug,
			})
		}

		// Check if there's a next page
		if orgsResp.Links.Next != "" {
			// If the next URL is a relative path, make it absolute
			if !isAbsoluteURL(orgsResp.Links.Next) {
				nextURL = c.RestBaseURL + orgsResp.Links.Next
			} else {
				nextURL = orgsResp.Links.Next
			}
		} else {
			// No more pages
			nextURL = ""
		}
	}

	return allOrganizations, nil
}

// isAbsoluteURL checks if the given URL is absolute (starts with http:// or https://)
func isAbsoluteURL(urlStr string) bool {
	return len(urlStr) > 8 && (urlStr[:7] == "http://" || urlStr[:8] == "https://")
}

// getSnykAPIToken retrieves the Snyk API token from the user's config
func getSnykAPIToken() (string, error) {
	cmd := exec.Command("snyk", "config", "get", "INTERNAL_OAUTH_TOKEN_STORAGE")
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to execute snyk config command: %w", err)
	}

	var tokenStorage struct {
		AccessToken  string `json:"access_token"`
		TokenType    string `json:"token_type"`
		RefreshToken string `json:"refresh_token"`
		Expiry       string `json:"expiry"`
	}

	if err := json.Unmarshal(output, &tokenStorage); err != nil {
		return "", fmt.Errorf("failed to unmarshal token storage: %w", err)
	}

	if tokenStorage.AccessToken == "" {
		return "", fmt.Errorf("no access token found in Snyk config")
	}

	return tokenStorage.AccessToken, nil
}

// GetTargetsWithURL retrieves targets for an organization with a specific URL
func (c *SnykClient) GetTargetsWithURL(orgID string, urlFilter string) ([]Target, error) {
	params := url.Values{}
	params.Add("version", SnykAPIRestVersion)
	params.Add("limit", fmt.Sprintf("%d", c.PageLimit))
	if urlFilter != "" {
		params.Add("url", urlFilter)
	}

	reqURL := fmt.Sprintf("%s/orgs/%s/targets?%s", c.RestBaseURL, orgID, params.Encode())

	// Call the helper function to fetch all paginated results
	targets, err := c.getAllTargetPages(reqURL)
	if err != nil {
		return nil, err
	}

	return targets, nil
}

// getAllTargetPages retrieves all pages of targets from the Snyk REST API
func (c *SnykClient) getAllTargetPages(initialURL string) ([]Target, error) {
	var allTargets []Target
	nextURL := initialURL

	for nextURL != "" {
		// Log the request
		c.logRequest("GET", nextURL)

		// Make request to the current URL
		req, err := http.NewRequest("GET", nextURL, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to create request: %w", err)
		}

		req.Header.Set("Content-Type", "application/vnd.api+json")
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.APIToken))

		resp, err := c.HTTPClient.Do(req)
		if err != nil {
			return nil, fmt.Errorf("failed to execute request: %w", err)
		}

		if resp.StatusCode != http.StatusOK {
			bodyBytes, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			return nil, fmt.Errorf("unexpected status code: %d, body: %s", resp.StatusCode, string(bodyBytes))
		}

		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			return nil, fmt.Errorf("failed to read response body: %w", err)
		}

		var targetsResp TargetsResponse
		if err := json.Unmarshal(body, &targetsResp); err != nil {
			return nil, fmt.Errorf("failed to unmarshal response: %w", err)
		}

		// Append targets from this page to our result
		allTargets = append(allTargets, targetsResp.Data...)

		// Check if there's a next page
		if targetsResp.Links.Next != "" {
			// If the next URL is a relative path, make it absolute
			if !isAbsoluteURL(targetsResp.Links.Next) {
				nextURL = c.RestBaseURL + targetsResp.Links.Next
			} else {
				nextURL = targetsResp.Links.Next
			}
		} else {
			// No more pages
			nextURL = ""
		}
	}

	return allTargets, nil
}

// GetTargets retrieves all targets for an organization
func (c *SnykClient) GetTargets(orgID string) ([]Target, error) {
	return c.GetTargetsWithURL(orgID, "")
}

// FindOrgWithTargetURL finds an organization with a target matching the given URL
func (c *SnykClient) FindOrgWithTargetURL(targetURL string) (*OrgTarget, error) {
	organizations, err := c.GetOrganizations()
	if err != nil {
		return nil, fmt.Errorf("failed to get organizations: %w", err)
	}

	// Create both HTTP and HTTPS variants of the URL to query
	httpVariant := targetURL
	httpsVariant := targetURL

	// Make sure we have both variants of the URL
	if strings.HasPrefix(targetURL, "https://") {
		httpVariant = "http://" + strings.TrimPrefix(targetURL, "https://")
	} else if strings.HasPrefix(targetURL, "http://") {
		httpsVariant = "https://" + strings.TrimPrefix(targetURL, "http://")
	} else {
		// If no protocol provided, default to both http:// and https:// prefixes
		httpVariant = "http://" + targetURL
		httpsVariant = "https://" + targetURL
	}

	for _, org := range organizations {
		// Get all targets for this organization
		targets, err := c.GetTargets(org.ID)
		if err != nil {
			// Continue to next org on error
			continue
		}

		// Search for matching URL in the targets
		for _, target := range targets {
			if target.Attributes.URL == httpVariant || target.Attributes.URL == httpsVariant {
				// Found a target matching one of the URL variants
				return &OrgTarget{
					OrgID:      org.ID,
					OrgName:    org.Name,
					TargetURL:  target.Attributes.URL,
					TargetName: target.Attributes.DisplayName,
				}, nil
			}
		}
	}

	return nil, fmt.Errorf("no organization found with a target matching URL: %s", targetURL)
}
