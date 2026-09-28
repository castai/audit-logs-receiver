package auditlogsreceiver

import (
	"encoding/base64"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
)

func getAttr(t *testing.T, attrs pcommon.Map, key string) pcommon.Value {
	t.Helper()
	v, ok := attrs.Get(key)
	require.True(t, ok, "expected attribute %q to exist", key)

	return v
}

func TestDecodeEventBody(t *testing.T) {
	changelogJSON := `{"changelog":[{"field":"Autoscaler.enabled","oldValue":true,"newValue":false}]}`
	changelogB64 := base64.StdEncoding.EncodeToString([]byte(changelogJSON))

	tests := []struct {
		name     string
		input    string
		expected map[string]any
	}{
		{
			name:  "base64-encoded JSON",
			input: changelogB64,
			expected: map[string]any{
				"changelog": []any{
					map[string]any{
						"field":    "Autoscaler.enabled",
						"oldValue": true,
						"newValue": false,
					},
				},
			},
		},
		{
			name:  "plain JSON (future proto string)",
			input: `{"cluster":{"id":"abc-123"}}`,
			expected: map[string]any{
				"cluster": map[string]any{"id": "abc-123"},
			},
		},
		{
			name:     "valid base64 but not JSON",
			input:    base64.StdEncoding.EncodeToString([]byte("not json at all")),
			expected: map[string]any{"_raw": "not json at all"},
		},
		{
			name:     "not base64, not JSON",
			input:    "plain text with spaces and !symbols",
			expected: map[string]any{"_raw": "plain text with spaces and !symbols"},
		},
		{
			name:     "empty string",
			input:    "",
			expected: map[string]any{"_raw": ""},
		},
		{
			name:     "empty JSON object",
			input:    base64.StdEncoding.EncodeToString([]byte("{}")),
			expected: map[string]any{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := decodeEventBody(tt.input)
			assert.Equal(t, tt.expected, got)
		})
	}
}

func TestEventToLogRecordWithBody(t *testing.T) {
	bodyB64 := base64.StdEncoding.EncodeToString([]byte(`{"changelog":[{"field":"Autoscaler.enabled"}]}`))

	e := Event{
		EventID:       "event-body-001",
		TenantID:      "tenant-001",
		OccurredAt:    time.Now().UTC(),
		IngestedAt:    time.Now().UTC(),
		Body:           bodyB64,
	}

	lr, err := e.ToLogRecord()
	require.NoError(t, err)

	// event.body should be a structured map, not a base64 string.
	bodyAttr := getAttr(t, lr.Attributes(), "event.body")
	_, ok := bodyAttr.Map().Get("changelog")
	assert.True(t, ok, "expected changelog key in event.body")
}

func TestEventToLogRecordWithoutBody(t *testing.T) {
	e := Event{
		EventID:           "event-nobody-001",
		TenantID:          "tenant-001",
		OccurredAt:        time.Now().UTC(),
		IngestedAt:        time.Now().UTC(),
		EventDomain:       "autoscaler",
		EventResource:     "policy",
		EventAction:       "updated",
		EventSeverity:     9,
		EventSeverityText: "info",
		Description:       "Policy updated",
		Body:              "", // No body.
	}

	lr, err := e.ToLogRecord()
	require.NoError(t, err)

	// The event.body attribute should be absent when Body is empty.
	_, ok := lr.Attributes().Get("event.body")
	assert.False(t, ok, "event.body should not be set when Body is empty")
}

func TestEventToLogRecord(t *testing.T) {
	occurredAt := time.Now().UTC().Add(-time.Hour)
	ingestedAt := time.Now().UTC()
	correlationID := "a1b2c3d4-e5f6-7890-abcd-ef1234567890"

	e := Event{
		EventID:              "event-0001",
		CorrelationID:        correlationID,
		TenantID:             "tenant-001",
		OccurredAt:           occurredAt,
		IngestedAt:           ingestedAt,
		EventDomain:          "autoscaler",
		EventResource:        "node",
		EventAction:          "added",
		EventSeverity:        9,
		EventSeverityText:    "info",
		Description:          "Node added",
		ClusterID:            "cluster-123",
		CorrelatedEventCount: 2,
		Actor: &Actor{
			Type:        "user",
			ID:          "user-001",
			DisplayName: "Alice",
			Email:       "alice@example.com",
		},
		Resource: &Resource{
			Type:        "cluster",
			ID:          "cluster-123",
			DisplayName: "prod-cluster",
		},
		Labels: map[string]any{
			"env":    "prod",
			"region": "us-east-1",
		},
	}

	lr, err := e.ToLogRecord()
	require.NoError(t, err)

	assert.EqualValues(t, occurredAt.UnixNano(), lr.Timestamp().AsTime().UnixNano())
	assert.Equal(t, "Node added", lr.Body().AsString())
	assert.EqualValues(t, 9, lr.SeverityNumber())
	assert.Equal(t, "info", lr.SeverityText())

	attrs := lr.Attributes()
	assert.Equal(t, "tenant-001", getAttr(t, attrs, "tenant.id").Str())
	assert.Equal(t, "event-0001", getAttr(t, attrs, "event.id").Str())
	assert.Equal(t, "autoscaler", getAttr(t, attrs, "event.domain").Str())
	assert.Equal(t, "node", getAttr(t, attrs, "event.resource").Str())
	assert.Equal(t, "added", getAttr(t, attrs, "event.action").Str())

	assert.Equal(t, correlationID, getAttr(t, attrs, "correlation.id").Str())
	assert.EqualValues(t, 2, getAttr(t, attrs, "correlation.count").Int())

	assert.False(t, lr.TraceID().IsEmpty())
	assert.Equal(t, "a1b2c3d4e5f67890abcdef1234567890", lr.TraceID().String())

	assert.Equal(t, "user-001", getAttr(t, attrs, "actor.id").Str())
	assert.Equal(t, "user", getAttr(t, attrs, "actor.type").Str())
	assert.Equal(t, "Alice", getAttr(t, attrs, "actor.display_name").Str())
	assert.Equal(t, "alice@example.com", getAttr(t, attrs, "actor.email").Str())

	assert.Equal(t, "cluster", getAttr(t, attrs, "resource.type").Str())
	assert.Equal(t, "cluster-123", getAttr(t, attrs, "resource.id").Str())
	assert.Equal(t, "prod-cluster", getAttr(t, attrs, "resource.display_name").Str())

	assert.Equal(t, "tenant-001", getAttr(t, attrs, "tenant.id").Str())
	assert.Equal(t, "cluster-123", getAttr(t, attrs, "cluster.id").Str())

	assert.Equal(t, ingestedAt.UTC().Format(time.RFC3339Nano), getAttr(t, attrs, "ingested_at").Str())

	labels := getAttr(t, attrs, "labels").Map()
	require.Equal(t, 2, labels.Len())
	assert.Equal(t, "prod", getAttr(t, labels, "env").Str())
	assert.Equal(t, "us-east-1", getAttr(t, labels, "region").Str())
}
