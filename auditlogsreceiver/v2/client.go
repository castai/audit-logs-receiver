package auditlogsreceiver

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const (
	auditEventsPath      = "/v2/audit/events"
	defaultClientTimeout = 30 * time.Second
)

// Client is an HTTP client for the CAST AI Audit v2 API.
type Client struct {
	httpClient *http.Client
	baseURL    string
	apiKey     string
}

// WithTimeout sets the HTTP client timeout.
func WithTimeout(d time.Duration) func(*Client) {
	return func(c *Client) {
		c.httpClient.Timeout = d
	}
}

// WithHTTPClient sets a custom HTTP client.
func WithHTTPClient(h *http.Client) func(*Client) {
	return func(c *Client) {
		c.httpClient = h
	}
}

// NewClient creates a new Audit v2 API client.
func NewClient(baseURL, apiKey string, opts ...func(*Client)) *Client {
	c := &Client{
		httpClient: &http.Client{Timeout: defaultClientTimeout},
		baseURL:    strings.TrimSuffix(baseURL, "/"),
		apiKey:     apiKey,
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// Actor represents the entity that initiated the event.
type Actor struct {
	Type        string `json:"type,omitempty"`
	ID          string `json:"id,omitempty"`
	DisplayName string `json:"displayName,omitempty"`
	Email       string `json:"email,omitempty"`
}

// Resource represents the entity affected by the event.
type Resource struct {
	Type        string `json:"type,omitempty"`
	ID          string `json:"id,omitempty"`
	DisplayName string `json:"displayName,omitempty"`
}

// Event represents a single event from the v2 API.
type Event struct {
	EventID              string         `json:"eventId,omitempty"`
	RequestID            string         `json:"requestId,omitempty"`
	CorrelationID        string         `json:"correlationId,omitempty"`
	TenantID             string         `json:"tenantId,omitempty"`
	OccurredAt           time.Time      `json:"occurredAt,omitempty"`
	IngestedAt           time.Time      `json:"ingestedAt,omitempty"`
	SourceService        string         `json:"sourceService,omitempty"`
	SourceVersion        string         `json:"sourceVersion,omitempty"`
	EventDomain          string         `json:"eventDomain,omitempty"`
	EventResource        string         `json:"eventResource,omitempty"`
	EventAction          string         `json:"eventAction,omitempty"`
	EventSeverity        uint32         `json:"eventSeverity,omitempty"`
	EventSeverityText    string         `json:"eventSeverityText,omitempty"`
	Description          string         `json:"description,omitempty"`
	Actor                *Actor         `json:"actor,omitempty"`
	Resource             *Resource      `json:"resource,omitempty"`
	Labels               map[string]any `json:"labels,omitempty"`
	ClusterID            string         `json:"clusterId,omitempty"`
	CorrelatedEventCount int32          `json:"correlatedEventCount,omitempty"`
}

// ListEventsResponse is the response from the list audit events endpoint.
type ListEventsResponse struct {
	Events         []Event `json:"events"`
	NextCursor     string  `json:"nextCursor,omitempty"`
	PreviousCursor string  `json:"previousCursor,omitempty"`
	Count          int32   `json:"count,omitempty"`
}

// Filters defines the criteria for audit event queries.
type Filters struct {
	Search    string   `json:"search,omitempty"`
	Clusters  []string `json:"clusters,omitempty"`
	Domains   []string `json:"domains,omitempty"`
	Resources []string `json:"resources,omitempty"`
	Actions   []string `json:"actions,omitempty"`
	Sources   []string `json:"sources,omitempty"`
	Severity  []string `json:"severity,omitempty"`
}

// ListEventsParams defines the parameters for listing audit events.
type ListEventsParams struct {
	PageLimit  int
	PageCursor string
	FromDate   time.Time
	ToDate     time.Time
	Filters    Filters
}

// ListEvents retrieves audit events from the v2 API.
func (c *Client) ListEvents(ctx context.Context, params ListEventsParams) (*ListEventsResponse, error) {
	reqURL := c.baseURL + auditEventsPath

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	req.URL.RawQuery = buildQuery(params).Encode()
	req.Header.Set("X-API-Key", c.apiKey)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "castai/audit-logs-receiver/0.2.0")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("sending request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response body: %w", err)
	}

	if statusCode := resp.StatusCode; statusCode < 200 || statusCode >= 300 {
		return nil, fmt.Errorf("unexpected status code %d: %s", statusCode, http.StatusText(statusCode))
	}

	var result ListEventsResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("unmarshaling response: %w", err)
	}

	return &result, nil
}

func buildQuery(params ListEventsParams) url.Values {
	q := url.Values{}

	if params.PageLimit > 0 {
		q.Set("page.limit", strconv.Itoa(params.PageLimit))
	}
	if params.PageCursor != "" {
		q.Set("page.cursor", params.PageCursor)
	}
	if !params.FromDate.IsZero() {
		q.Set("fromDate", params.FromDate.UTC().Format(time.RFC3339Nano))
	}
	if !params.ToDate.IsZero() {
		q.Set("toDate", params.ToDate.UTC().Format(time.RFC3339Nano))
	}

	f := params.Filters
	if f.Search != "" {
		q.Set("filter.search", f.Search)
	}
	if len(f.Clusters) > 0 {
		q["filter.clusters"] = f.Clusters
	}
	if len(f.Domains) > 0 {
		q["filter.domains"] = f.Domains
	}
	if len(f.Resources) > 0 {
		q["filter.resources"] = f.Resources
	}
	if len(f.Actions) > 0 {
		q["filter.actions"] = f.Actions
	}
	if len(f.Sources) > 0 {
		q["filter.sources"] = f.Sources
	}
	if len(f.Severity) > 0 {
		q["filter.severity"] = f.Severity
	}

	q.Set("sort.field", "occurred_at")
	q.Set("sort.order", "ASC")

	return q
}
