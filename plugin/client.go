package plugin

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type connectClient struct {
	baseURL    *url.URL
	httpClient *http.Client
	token      string
}

func newConnectClient(baseURL, token string, httpClient *http.Client) (*connectClient, error) {
	if baseURL == "" {
		return nil, fmt.Errorf("missing 1Password Connect host")
	}
	if token == "" {
		return nil, fmt.Errorf("missing 1Password Connect token")
	}
	parsed, err := url.Parse(baseURL)
	if err != nil {
		return nil, fmt.Errorf("invalid 1Password Connect host: %w", err)
	}
	if parsed.Scheme == "" || parsed.Host == "" {
		return nil, fmt.Errorf("1Password Connect host must include scheme and host")
	}
	cleanPath := strings.TrimSuffix(parsed.Path, "/")
	if cleanPath == "" {
		cleanPath = "/v1"
	} else if !strings.HasSuffix(cleanPath, "/v1") {
		cleanPath = cleanPath + "/v1"
	}
	parsed.Path = cleanPath
	parsed.RawQuery = ""
	parsed.Fragment = ""

	client := httpClient
	if client == nil {
		client = &http.Client{Timeout: 15 * time.Second}
	}

	return &connectClient{
		baseURL:    parsed,
		httpClient: client,
		token:      token,
	}, nil
}

func (c *connectClient) findVaultByName(ctx context.Context, name string) (*vaultSummary, error) {
	var vaults []vaultSummary
	err := c.get(ctx, "vaults", url.Values{
		"filter": {buildEqualsFilter("name", name)},
	}, &vaults)
	if err != nil {
		return nil, err
	}
	if len(vaults) == 0 {
		return nil, fmt.Errorf("vault %q not found", name)
	}
	if len(vaults) > 1 {
		return nil, fmt.Errorf("multiple vaults match name %q", name)
	}
	return &vaults[0], nil
}

func (c *connectClient) findItemByTitle(ctx context.Context, vaultID, title string) (*itemSummary, error) {
	var items []itemSummary
	path := fmt.Sprintf("vaults/%s/items", escapePathSegment(vaultID))
	err := c.get(ctx, path, url.Values{
		"filter": {buildEqualsFilter("title", title)},
	}, &items)
	if err != nil {
		return nil, err
	}
	if len(items) == 0 {
		return nil, fmt.Errorf("item %q not found in vault %q", title, vaultID)
	}
	if len(items) > 1 {
		return nil, fmt.Errorf("multiple items named %q found in vault %q", title, vaultID)
	}
	return &items[0], nil
}

func (c *connectClient) getItem(ctx context.Context, vaultID, itemID string) (*fullItem, error) {
	var item fullItem
	path := fmt.Sprintf("vaults/%s/items/%s", escapePathSegment(vaultID), escapePathSegment(itemID))
	if err := c.get(ctx, path, nil, &item); err != nil {
		return nil, err
	}
	return &item, nil
}

func (c *connectClient) get(ctx context.Context, path string, query url.Values, out interface{}) error {
	u := *c.baseURL
	basePath := strings.TrimSuffix(c.baseURL.Path, "/")
	relative := strings.TrimPrefix(path, "/")
	if relative != "" {
		u.Path = basePath + "/" + relative
	} else {
		u.Path = basePath
	}
	if len(query) > 0 {
		u.RawQuery = query.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		if out == nil {
			return nil
		}
		return json.NewDecoder(resp.Body).Decode(out)
	}

	apiErr := &apiError{StatusCode: resp.StatusCode, Message: resp.Status}
	var payload struct {
		Status  int    `json:"status"`
		Message string `json:"message"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err == nil && payload.Message != "" {
		apiErr.Message = payload.Message
	}
	return apiErr
}

type apiError struct {
	StatusCode int
	Message    string
}

func (e *apiError) Error() string {
	return fmt.Sprintf("1Password Connect error (%d): %s", e.StatusCode, e.Message)
}

func buildEqualsFilter(field, value string) string {
	return fmt.Sprintf(`%s eq "%s"`, field, escapeFilterValue(value))
}

func escapeFilterValue(value string) string {
	replaced := strings.Replace(value, `\`, `\\`, -1)
	replaced = strings.Replace(replaced, `"`, `\"`, -1)
	return replaced
}

func escapePathSegment(segment string) string {
	return strings.Replace(url.PathEscape(segment), "+", "%20", -1)
}
