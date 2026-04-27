package osv

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// VulnInfo is a single vulnerability record from OSV.
type VulnInfo struct {
	ID            string `json:"id"`
	Summary       string `json:"summary"`
	Severity      string `json:"severity"`
	AffectedRange string `json:"affected_range"`
	FixedIn       string `json:"fixed_in,omitempty"`
	URL           string `json:"url"`
}

// osvQueryRequest is the POST body sent to the OSV API.
type osvQueryRequest struct {
	Version string `json:"version,omitempty"`
	Package struct {
		Name      string `json:"name"`
		Ecosystem string `json:"ecosystem"`
	} `json:"package"`
}

// osvResponse is the top-level JSON response from OSV.
type osvResponse struct {
	Vulns []struct {
		ID       string `json:"id"`
		Summary  string `json:"summary"`
		Severity []struct {
			Type  string `json:"type"`
			Score string `json:"score"`
		} `json:"severity"`
		Affected []struct {
			Ranges []struct {
				Type   string `json:"type"`
				Events []struct {
					Introduced string `json:"introduced,omitempty"`
					Fixed      string `json:"fixed,omitempty"`
				} `json:"events"`
			} `json:"ranges"`
		} `json:"affected"`
		References []struct {
			URL string `json:"url"`
		} `json:"references"`
	} `json:"vulns"`
}

// Client queries the OSV.dev vulnerability database.
type Client struct {
	baseURL    string
	httpClient *http.Client
}

// Option configures a Client.
type Option func(*Client)

// WithBaseURL overrides the OSV API base URL (for testing).
func WithBaseURL(url string) Option {
	return func(c *Client) { c.baseURL = url }
}

// NewClient creates a new OSV client.
func NewClient(opts ...Option) *Client {
	c := &Client{
		baseURL:    "https://api.osv.dev",
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// FetchVulns returns vulnerabilities for the named PyPI package. When version
// is non-empty only vulnerabilities affecting that specific version are returned;
// otherwise all historical vulnerabilities are returned.
func (c *Client) FetchVulns(ctx context.Context, name, version string) ([]VulnInfo, error) {
	var body osvQueryRequest
	body.Version = version
	body.Package.Name = name
	body.Package.Ecosystem = "PyPI"

	payload, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("osv: encode request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/v1/query", bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("osv: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("osv: request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("osv: unexpected status %d", resp.StatusCode)
	}

	var result osvResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("osv: decode response: %w", err)
	}

	vulns := make([]VulnInfo, 0, len(result.Vulns))
	for _, v := range result.Vulns {
		vi := VulnInfo{
			ID:      v.ID,
			Summary: v.Summary,
		}

		// Extract severity from the first entry.
		if len(v.Severity) > 0 {
			vi.Severity = v.Severity[0].Score
		}
		if vi.Severity == "" {
			vi.Severity = "Unknown"
		}

		// Extract affected range and fixed version from the first ECOSYSTEM range.
		if len(v.Affected) > 0 {
			for _, r := range v.Affected[0].Ranges {
				if r.Type != "ECOSYSTEM" {
					continue
				}
				var introduced, fixed string
				for _, e := range r.Events {
					if e.Introduced != "" {
						introduced = e.Introduced
					}
					if e.Fixed != "" {
						fixed = e.Fixed
					}
				}
				if introduced != "" {
					vi.AffectedRange = ">=" + introduced
					if fixed != "" {
						vi.AffectedRange += ", <" + fixed
					}
				}
				if fixed != "" {
					vi.FixedIn = fixed
				}
				break
			}
		}

		// Use first reference URL.
		if len(v.References) > 0 {
			vi.URL = v.References[0].URL
		}

		vulns = append(vulns, vi)
	}
	return vulns, nil
}
