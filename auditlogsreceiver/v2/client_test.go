package auditlogsreceiver

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestListEventsQueryParams(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "castai/audit-logs-receiver/0.2.0", r.Header.Get("User-Agent"))

		u, err := url.Parse(r.URL.String())
		require.NoError(t, err)

		q := u.Query()
		assert.Equal(t, "100", q.Get("page.limit"))
		assert.Equal(t, "test-query", q.Get("filter.search"))
		assert.Equal(t, "cluster-1,cluster-2", q.Get("filter.clusters"))
		assert.Equal(t, "platform,autoscaler", q.Get("filter.domains"))
		assert.Equal(t, "cluster,node", q.Get("filter.resources"))
		assert.Equal(t, "created,deleted", q.Get("filter.actions"))
		assert.Equal(t, "provisioner", q.Get("filter.sources"))
		assert.Equal(t, "info,error", q.Get("filter.severity"))
		assert.Equal(t, "occurred_at", q.Get("sort.field"))
		assert.Equal(t, "ASC", q.Get("sort.order"))
		assert.NotEmpty(t, q.Get("fromDate"))
		assert.NotEmpty(t, q.Get("toDate"))

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		require.NoError(t, json.NewEncoder(w).Encode(ListEventsResponse{}))
	}))
	t.Cleanup(ts.Close)

	c := NewClient(ts.URL, "test-key")
	_, err := c.ListEvents(context.Background(), ListEventsParams{
		PageLimit: 100,
		FromDate:  time.Date(2026, 7, 14, 10, 0, 0, 0, time.UTC),
		ToDate:    time.Date(2026, 7, 14, 11, 0, 0, 0, time.UTC),
		Filters: Filters{
			Search:    "test-query",
			Clusters:  []string{"cluster-1", "cluster-2"},
			Domains:   []string{"platform", "autoscaler"},
			Resources: []string{"cluster", "node"},
			Actions:   []string{"created", "deleted"},
			Sources:   []string{"provisioner"},
			Severity:  []string{"info", "error"},
		},
	})
	require.NoError(t, err)
}

func TestListEventsCursorParam(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u, err := url.Parse(r.URL.String())
		require.NoError(t, err)

		q := u.Query()
		assert.Equal(t, "50", q.Get("page.limit"))
		assert.Equal(t, "abc123", q.Get("page.cursor"))

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		require.NoError(t, json.NewEncoder(w).Encode(ListEventsResponse{}))
	}))
	t.Cleanup(ts.Close)

	c := NewClient(ts.URL, "test-key")
	_, err := c.ListEvents(context.Background(), ListEventsParams{
		PageLimit:  50,
		PageCursor: "abc123",
	})
	require.NoError(t, err)
}

func TestListEventsAuthHeader(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "very-secret-key", r.Header.Get("X-API-Key"))

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		require.NoError(t, json.NewEncoder(w).Encode(ListEventsResponse{}))
	}))
	t.Cleanup(ts.Close)

	c := NewClient(ts.URL, "very-secret-key")
	_, err := c.ListEvents(context.Background(), ListEventsParams{PageLimit: 100})
	require.NoError(t, err)
}

func TestListEventsResponse(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{
			"events": [
				{
					"eventId": "evt-001",
					"correlationId": "corr-001",
					"tenantId": "tenant-001",
					"occurredAt": "2026-07-14T10:30:00.123456789Z",
					"ingestedAt": "2026-07-14T10:30:01Z",
					"eventDomain": "platform",
					"eventResource": "cluster",
					"eventAction": "created",
					"eventSeverity": 9,
					"eventSeverityText": "info",
					"description": "Cluster created",
					"clusterId": "cluster-123",
					"correlatedEventCount": 2,
					"actor": {
						"type": "user",
						"id": "user-001",
						"displayName": "Alice",
						"email": "alice@example.com"
					},
					"resource": {
						"type": "cluster",
						"id": "cluster-123",
						"displayName": "prod-cluster"
					},
					"labels": {
						"env": "prod",
						"region": "us-east-1"
					}
				}
			],
			"nextCursor": "cursor-abc",
			"count": 1
		}`))
	}))
	t.Cleanup(ts.Close)

	c := NewClient(ts.URL, "test-key")
	resp, err := c.ListEvents(context.Background(), ListEventsParams{PageLimit: 100})
	require.NoError(t, err)

	require.NotNil(t, resp.Events)
	require.Len(t, resp.Events, 1)

	e := resp.Events[0]
	assert.Equal(t, "evt-001", e.EventID)
	assert.Equal(t, "corr-001", e.CorrelationID)
	assert.Equal(t, "tenant-001", e.TenantID)
	assert.Equal(t, "platform", e.EventDomain)
	assert.Equal(t, "cluster", e.EventResource)
	assert.Equal(t, "created", e.EventAction)
	assert.EqualValues(t, 9, e.EventSeverity)
	assert.Equal(t, "info", e.EventSeverityText)
	assert.Equal(t, "Cluster created", e.Description)
	assert.Equal(t, "cluster-123", e.ClusterID)
	assert.EqualValues(t, 2, e.CorrelatedEventCount)

	assert.False(t, e.OccurredAt.IsZero())
	assert.Equal(t, "2026-07-14T10:30:00.123456789Z", e.OccurredAt.UTC().Format(time.RFC3339Nano))
	assert.False(t, e.IngestedAt.IsZero())

	require.NotNil(t, e.Actor)
	assert.Equal(t, "user", e.Actor.Type)
	assert.Equal(t, "user-001", e.Actor.ID)
	assert.Equal(t, "Alice", e.Actor.DisplayName)
	assert.Equal(t, "alice@example.com", e.Actor.Email)

	require.NotNil(t, e.Resource)
	assert.Equal(t, "cluster", e.Resource.Type)
	assert.Equal(t, "cluster-123", e.Resource.ID)
	assert.Equal(t, "prod-cluster", e.Resource.DisplayName)

	require.Len(t, e.Labels, 2)
	assert.Equal(t, "prod", e.Labels["env"])
	assert.Equal(t, "us-east-1", e.Labels["region"])

	assert.Equal(t, "cursor-abc", resp.NextCursor)
	assert.EqualValues(t, 1, resp.Count)
}

func TestListEventsEmptyResponse(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"events": []}`))
	}))
	t.Cleanup(ts.Close)

	c := NewClient(ts.URL, "test-key")
	resp, err := c.ListEvents(context.Background(), ListEventsParams{PageLimit: 100})
	require.NoError(t, err)

	assert.Empty(t, resp.Events)
	assert.Empty(t, resp.NextCursor)
}

func TestListEventsErrorResponse(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error": "invalid api key"}`))
	}))
	t.Cleanup(ts.Close)

	c := NewClient(ts.URL, "invalid-key")
	_, err := c.ListEvents(context.Background(), ListEventsParams{PageLimit: 100})
	require.Error(t, err)
	require.ErrorContains(t, err, "Unauthorized")
}

func TestListEventsNoParams(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"events": []}`))
	}))
	t.Cleanup(ts.Close)

	c := NewClient(ts.URL, "test-key")
	resp, err := c.ListEvents(context.Background(), ListEventsParams{})
	require.NoError(t, err)
	require.NotNil(t, resp)
}

func TestListEventsServerError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	t.Cleanup(ts.Close)

	c := NewClient(ts.URL, "test-key")
	_, err := c.ListEvents(context.Background(), ListEventsParams{PageLimit: 100})
	require.Error(t, err)
	require.ErrorContains(t, err, "unexpected status code")
}

func TestListEventsTimeout(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(50 * time.Millisecond)

		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"events": []}`))
	}))
	t.Cleanup(ts.Close)

	c := NewClient(ts.URL, "test-key", WithTimeout(20*time.Millisecond))
	_, err := c.ListEvents(context.Background(), ListEventsParams{PageLimit: 100})
	require.Error(t, err)
	require.ErrorContains(t, err, "context deadline exceeded")
}
