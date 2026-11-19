package auditlogsreceiver

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/castai/audit-logs-receiver/audit-logs/v2/checkpoint"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/plog"
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

// TestReceiverRestartMidPagination verifies that a restart while pagination is
// in progress resumes from the persisted cursor instead of re-fetching the
// first page (no duplicates, no skipped pages).
func TestReceiverRestartMidPagination(t *testing.T) {
	var mu sync.Mutex
	var gotCursors []string

	// The receiver is expected to issue exactly two requests: page 1 (no
	// cursor) in phase 1 and the cursor-1 page in phase 2. Any further request
	// (the poll loop keeps running until Shutdown) is parked until its request
	// context is cancelled, so it can never add a cursor or record before the
	// assertions below run.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		if len(gotCursors) >= 2 {
			mu.Unlock()
			<-r.Context().Done() // Park the request until shutdown cancels it.
			return
		}
		cursor := r.URL.Query().Get("page.cursor")
		gotCursors = append(gotCursors, cursor)
		mu.Unlock()

		resp := ListEventsResponse{
			Events: []Event{{EventID: "event-page1", Description: "page 1", OccurredAt: time.Now().UTC()}},
			Count:  1,
		}
		if cursor == "" {
			resp.NextCursor = "cursor-1"
		} else {
			resp.Events = []Event{{EventID: "event-page2", Description: "page 2", OccurredAt: time.Now().UTC()}}
		}

		w.Header().Set("Content-Type", "application/json")
		require.NoError(t, json.NewEncoder(w).Encode(resp))
	}))
	t.Cleanup(server.Close)

	filename := filepath.Join(t.TempDir(), "checkpoint.json")

	// Phase 1: poll page 1, receive a cursor, then "crash" (shutdown).
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
	require.Eventually(t, func() bool { return sink.LogRecordCount() == 1 }, time.Second, 5*time.Millisecond)
	require.NoError(t, r.Shutdown(context.Background()))

	// Checkpoint must hold the mid-pagination cursor.
	cp, err := checkpoint.NewFile(filename)
	require.NoError(t, err)
	require.Equal(t, "cursor-1", cp.Get().Cursor)

	// Phase 2: restart with the same checkpoint file. It must resume from
	// cursor-1, not re-fetch page 1 (no duplicate event-page1).
	sink2 := &consumertest.LogsSink{}
	r2, err := CreateReceiver(t.Context(), receiver.Settings{TelemetrySettings: component.TelemetrySettings{Logger: zap.NewNop()}}, config, sink2)
	require.NoError(t, err)
	require.NoError(t, r2.Start(context.Background(), nil))
	// Wait for the page-2 record AND for the poll cycle to complete (the
	// checkpoint write happens after ConsumeLogs). Then stop the receiver
	// before it can open a new interval.
	require.Eventually(t, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return sink2.LogRecordCount() == 1 && len(gotCursors) == 2
	}, time.Second, 5*time.Millisecond)
	require.NoError(t, r2.Shutdown(context.Background()))

	// No duplicates across the restart: the second receiver only ever saw the
	// cursor-1 page, and any further request was parked until shutdown.
	mu.Lock()
	defer mu.Unlock()
	require.Equal(t, []string{"", "cursor-1"}, gotCursors)
}

func TestReceiverEnterprise(t *testing.T) {
	events := []Event{
		{
			EventID:     "event-parent",
			TenantID:    "org-parent",
			Description: "Parent org event",
			OccurredAt:  time.Now().Add(-2 * time.Minute).UTC(),
		},
		{
			EventID:     "event-child",
			TenantID:    "org-child-a",
			Description: "Child org event",
			OccurredAt:  time.Now().Add(-time.Minute).UTC(),
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/v2/audit/enterprise/events", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(ListEventsResponse{
			Events: events,
			Count:  int32(len(events)),
		}))
	}))
	t.Cleanup(server.Close)

	sink := &consumertest.LogsSink{}
	config := &Config{
		API:          APIConfig{URL: server.URL, Key: "api.key", Timeout: defaultTimeout},
		PollInterval: 10 * time.Millisecond,
		PageLimit:    100,
		Lookback:     5 * time.Minute,
		Enterprise:   true,
	}

	rec, err := CreateReceiver(t.Context(), receiver.Settings{TelemetrySettings: component.TelemetrySettings{Logger: zap.NewNop()}}, config, sink)
	require.NoError(t, err)

	require.NoError(t, rec.Start(context.Background(), nil))
	require.Eventually(t, func() bool { return len(collectedEventIDs(sink)) >= 2 }, time.Second, 10*time.Millisecond)
	require.NoError(t, rec.Shutdown(context.Background()))

	byEventID := collectedEventIDs(sink)

	require.Contains(t, byEventID, "event-parent")
	assert.Equal(t, "org-parent", byEventID["event-parent"])

	require.Contains(t, byEventID, "event-child")
	assert.Equal(t, "org-child-a", byEventID["event-child"])
}

func TestReceiverEnterpriseModeSwitch(t *testing.T) {
	events := []Event{
		{
			EventID:     "event-parent",
			TenantID:    "org-parent",
			Description: "Parent org event",
			OccurredAt:  time.Now().Add(-2 * time.Minute).UTC(),
		},
		{
			EventID:     "event-child",
			TenantID:    "org-child-a",
			Description: "Child org event",
			OccurredAt:  time.Now().Add(-time.Minute).UTC(),
		},
	}

	var mu sync.Mutex
	var gotFroms []string

	var wantPath atomic.Value
	wantPath.Store("/v2/audit/events")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, wantPath.Load(), r.URL.Path)

		q := r.URL.Query()
		mu.Lock()
		gotFroms = append(gotFroms, q.Get("fromDate"))
		mu.Unlock()

		inWindow := []Event{}
		if from, err := time.Parse(time.RFC3339Nano, q.Get("fromDate")); err == nil {
			for _, ev := range events {
				if !ev.OccurredAt.Before(from) {
					inWindow = append(inWindow, ev)
				}
			}
		}
		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(ListEventsResponse{
			Events: inWindow,
			Count:  int32(len(inWindow)),
		}))
	}))
	t.Cleanup(server.Close)

	cpFile := filepath.Join(t.TempDir(), "checkpoint.json")

	sink1 := &consumertest.LogsSink{}
	rec, err := CreateReceiver(t.Context(), receiver.Settings{TelemetrySettings: component.TelemetrySettings{Logger: zap.NewNop()}}, &Config{
		API:            APIConfig{URL: server.URL, Key: "api.key", Timeout: defaultTimeout},
		PollInterval:   10 * time.Millisecond,
		PageLimit:      100,
		Lookback:       5 * time.Minute,
		CheckpointFile: cpFile,
	}, sink1)
	require.NoError(t, err)

	require.NoError(t, rec.Start(context.Background(), nil))
	require.Eventually(t, func() bool { return len(collectedEventIDs(sink1)) >= 2 }, time.Second, 10*time.Millisecond)
	require.NoError(t, rec.Shutdown(context.Background()))

	mu.Lock()
	phase1 := len(gotFroms)
	mu.Unlock()

	wantPath.Store("/v2/audit/enterprise/events")
	sink2 := &consumertest.LogsSink{}
	rec2, err := CreateReceiver(t.Context(), receiver.Settings{TelemetrySettings: component.TelemetrySettings{Logger: zap.NewNop()}}, &Config{
		API:            APIConfig{URL: server.URL, Key: "api.key", Timeout: defaultTimeout},
		PollInterval:   10 * time.Millisecond,
		PageLimit:      100,
		Lookback:       5 * time.Minute,
		CheckpointFile: cpFile,
		Enterprise:     true,
	}, sink2)
	require.NoError(t, err)

	require.NoError(t, rec2.Start(context.Background(), nil))
	require.Eventually(t, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return len(gotFroms) >= phase1+2
	}, time.Second, 10*time.Millisecond)
	require.NoError(t, rec2.Shutdown(context.Background()))

	assert.Empty(t, collectedEventIDs(sink2))

	mu.Lock()
	phase2Froms := append([]string(nil), gotFroms[phase1:]...)
	mu.Unlock()
	for _, from := range phase2Froms {
		parsed, err := time.Parse(time.RFC3339Nano, from)
		require.NoError(t, err)
		assert.True(t, parsed.After(events[1].OccurredAt), "fromDate %s should resume from the stored position", from)
	}
}

// collectedEventIDs indexes emitted log records by event.id, collapsing
// duplicates from re-polls.
func collectedEventIDs(sink *consumertest.LogsSink) map[string]string {
	byEventID := map[string]string{}
	for _, ld := range sink.AllLogs() {
		for i := 0; i < ld.ResourceLogs().Len(); i++ {
			rl := ld.ResourceLogs().At(i)
			for j := 0; j < rl.ScopeLogs().Len(); j++ {
				sl := rl.ScopeLogs().At(j)
				for k := 0; k < sl.LogRecords().Len(); k++ {
					lr := sl.LogRecords().At(k)
					eid, ok := lr.Attributes().Get("event.id")
					if !ok {
						continue
					}
					tid := ""
					if v, ok := lr.Attributes().Get("tenant.id"); ok {
						tid = v.AsString()
					}
					byEventID[eid.AsString()] = tid
				}
			}
		}
	}
	return byEventID
}

// errConsumer fails ConsumeLogs to simulate an exporter outage.
type errConsumer struct{ err error }

func (c errConsumer) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: false}
}

func (c errConsumer) ConsumeLogs(context.Context, plog.Logs) error { return c.err }

// TestReceiverCheckpointNotAdvancedOnConsumeError verifies the at-least-once
// guarantee: when the consumer fails, the checkpoint is not advanced, so the
// next poll retries the same page instead of dropping events.
func TestReceiverCheckpointNotAdvancedOnConsumeError(t *testing.T) {
	var polls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		polls.Add(1)
		w.Header().Set("Content-Type", "application/json")
		require.NoError(t, json.NewEncoder(w).Encode(ListEventsResponse{
			Events: []Event{{EventID: "event-0001", Description: "Event 1", OccurredAt: time.Now().UTC()}},
			Count:  1,
		}))
	}))
	t.Cleanup(server.Close)

	filename := filepath.Join(t.TempDir(), "checkpoint.json")
	config := &Config{
		API:            APIConfig{URL: server.URL, Key: "api.key", Timeout: defaultTimeout},
		PollInterval:   10 * time.Millisecond,
		PageLimit:      100,
		CheckpointFile: filename,
	}

	consumeErr := errors.New("exporter down")
	r, err := CreateReceiver(t.Context(), receiver.Settings{TelemetrySettings: component.TelemetrySettings{Logger: zap.NewNop()}}, config, errConsumer{err: consumeErr})
	require.NoError(t, err)
	require.NoError(t, r.Start(context.Background(), nil))
	defer func() { require.NoError(t, r.Shutdown(context.Background())) }()

	// Let several failed poll cycles pass.
	require.Eventually(t, func() bool { return polls.Load() >= 3 }, 5*time.Second, 10*time.Millisecond)

	// The checkpoint must never have been written: the poll interval [from, now)
	// is only advanced after a successful consume.
	_, statErr := os.Stat(filename)
	require.True(t, os.IsNotExist(statErr), "checkpoint file must not exist while consume fails, got err=%v", statErr)
}
