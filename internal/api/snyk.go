package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os/exec"
	"time"
)

const (
	SnykAPIRestBaseURL = "https://api.snyk.io/rest"
	SnykConfigPath     = ".config/configstore/snyk.json"
	SnykAPIRestVersion = "2024-10-15"
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
	Data []Organization `json:"data"`
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
	Data []Target `json:"data"`
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
	}, nil
}

// GetOrganizations retrieves the list of organizations from the Snyk REST API
func (c *SnykClient) GetOrganizations() ([]Organization, error) {
	params := url.Values{}
	params.Add("version", SnykAPIRestVersion)

	reqURL := fmt.Sprintf("%s/orgs?%s", c.RestBaseURL, params.Encode())
	req, err := http.NewRequest("GET", reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/vnd.api+json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.APIToken))

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("unexpected status code: %d, body: %s", resp.StatusCode, string(bodyBytes))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	var orgsResp OrgsResponse
	if err := json.Unmarshal(body, &orgsResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	// Map API response to Organization objects
	organizations := make([]Organization, len(orgsResp.Data))
	for i, org := range orgsResp.Data {
		organizations[i] = Organization{
			ID:   org.ID,
			Name: org.Attributes.Name,
			Slug: org.Attributes.Slug,
		}
	}

	return organizations, nil
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
	params.Add("url", urlFilter)

	reqURL := fmt.Sprintf("%s/orgs/%s/targets?%s", c.RestBaseURL, orgID, params.Encode())
	req, err := http.NewRequest("GET", reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/vnd.api+json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.APIToken))

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("unexpected status code: %d, body: %s", resp.StatusCode, string(bodyBytes))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	var targetsResp TargetsResponse
	if err := json.Unmarshal(body, &targetsResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return targetsResp.Data, nil
}

// FindOrgWithTargetURL finds an organization with a target matching the given URL
func (c *SnykClient) FindOrgWithTargetURL(targetURL string) (*OrgTarget, error) {
	organizations, err := c.GetOrganizations()
	if err != nil {
		return nil, fmt.Errorf("failed to get organizations: %w", err)
	}

	for _, org := range organizations {
		targets, err := c.GetTargetsWithURL(org.ID, targetURL)
		if err != nil {
			// Continue to next org on error
			continue
		}

		if len(targets) > 0 {
			// Found a target matching the URL
			return &OrgTarget{
				OrgID:      org.ID,
				OrgName:    org.Name,
				TargetURL:  targetURL,
				TargetName: targets[0].Attributes.DisplayName,
			}, nil
		}
	}

	return nil, fmt.Errorf("no organization found with a target matching URL: %s", targetURL)
}
