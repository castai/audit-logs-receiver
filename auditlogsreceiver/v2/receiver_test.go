package auditlogsreceiver

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/castai/audit-logs-receiver/audit-logs/v2/checkpoint"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/receiver"
	"go.uber.org/zap"
)

func TestReceiverNext(t *testing.T) {
	events := []Event{
		{EventID: "event-0001", Description: "Event 1", OccurredAt: time.Now().Add(-time.Minute).UTC()},
		{EventID: "event-0002", Description: "Event 2", OccurredAt: time.Now().Add(-time.Second).UTC()},
		{EventID: "event-0003", Description: "Event 3", OccurredAt: time.Now().UTC()},
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		require.NoError(t, json.NewEncoder(w).Encode(ListEventsResponse{
			Events:     events,
			Count:      3,
			NextCursor: "",
		}))
	}))
	t.Cleanup(server.Close)

	sink := &consumertest.LogsSink{}
	config := &Config{
		API:          APIConfig{URL: server.URL, Key: "api.key", Timeout: defaultTimeout},
		PollInterval: 10 * time.Millisecond,
		PageLimit:    100,
	}

	r, err := CreateReceiver(t.Context(), receiver.Settings{TelemetrySettings: component.TelemetrySettings{Logger: zap.NewNop()}}, config, sink)
	require.NoError(t, err)

	require.NoError(t, r.Start(context.Background(), nil))
	require.Eventually(t, func() bool { return sink.LogRecordCount() == 3 }, time.Second, 10*time.Millisecond)
	require.NoError(t, r.Shutdown(context.Background()))
}

func TestReceiverCheckpoint(t *testing.T) {
	events := []Event{
		{EventID: "event-0001", Description: "Event 1", OccurredAt: time.Now().Add(-time.Minute).UTC()},
		{EventID: "event-0002", Description: "Event 2", OccurredAt: time.Now().UTC()},
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		require.NoError(t, json.NewEncoder(w).Encode(ListEventsResponse{
			Events:     events,
			Count:      2,
			NextCursor: "",
		}))
	}))
	t.Cleanup(server.Close)

	filename := filepath.Join(t.TempDir(), "checkpoint.json")
	sink := &consumertest.LogsSink{}
	config := &Config{
		API:            APIConfig{URL: server.URL, Key: "api.key", Timeout: defaultTimeout},
		PollInterval:   10 * time.Millisecond,
		PageLimit:      100,
		CheckpointFile: filename,
	}

	r, err := CreateReceiver(t.Context(), receiver.Settings{TelemetrySettings: component.TelemetrySettings{Logger: zap.NewNop()}}, config, sink)
	require.NoError(t, err)

	require.NoError(t, r.Start(context.Background(), nil))
	require.Eventually(t, func() bool { return sink.LogRecordCount() == 2 }, time.Second, 10*time.Millisecond)
	require.NoError(t, r.Shutdown(context.Background()))

	checkpointer, err := checkpoint.NewFile(filename)
	require.NoError(t, err)

	state := checkpointer.Get()
	require.False(t, state.From.IsZero())
	require.True(t, state.From.After(events[len(events)-1].OccurredAt))
	require.True(t, state.To.IsZero())
	require.Empty(t, state.Cursor)
}
