// Package auditlogsreceiver implements v2 of the CAST.AI audit logs receiver.
package auditlogsreceiver

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/castai/audit-logs-receiver/audit-logs/v2/checkpoint"
	"github.com/castai/audit-logs-receiver/audit-logs/v2/internal/metadata"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"
	"go.uber.org/zap"
)

// Checkpointer can save and retrieve checkpoint state.
type Checkpointer interface {
	// Get retrieves the latest checkpoint.
	Get() checkpoint.State

	// Set persists the latest checkpoint.
	Set(checkpoint.State) error
}

// NewCheckpointer creates a checkpoint store from the configuration. If
// CheckpointFile is empty, an in-memory store is used; otherwise state is
// persisted to the given file.
func NewCheckpointer(conf *Config) (Checkpointer, error) {
	if conf.CheckpointFile == "" {
		return checkpoint.NewMemory(), nil
	}
	return checkpoint.NewFile(conf.CheckpointFile)
}

// Receiver should implement the OpenTelemetry logs receiver interface.
var _ receiver.Logs = (*Receiver)(nil)

// EventsLister lists audit events from an audit events source.
type EventsLister interface {
	ListEvents(ctx context.Context, params ListEventsParams) (*ListEventsResponse, error)
}

// Receiver is an audit logs receiver.
type Receiver struct {
	config      *Config
	lister      EventsLister
	consumer    consumer.Logs
	checkpoint  Checkpointer
	logger      *zap.Logger
	logsBuilder *metadata.LogsBuilder

	wg     sync.WaitGroup
	cancel context.CancelFunc
}

// Start tells the Receiver to begin polling for audit logs through the audit log
// API. An error is returned if the audit logs API cannot be reached, at which
// point the collector startup is aborted.
func (r *Receiver) Start(context.Context, component.Host) error {
	if r.cancel != nil {
		return nil // Already started.
	}

	ctx, cancel := context.WithCancel(context.Background())
	r.cancel = cancel
	r.wg.Go(func() {
		ticker := time.NewTicker(r.config.PollInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if err := r.Next(ctx); err != nil {
					r.logger.Error("failed to poll audit events", zap.Error(err))
				}
			}
		}
	})

	return nil
}

// Shutdown is invoked during service shutdown. The Receiver stops polling, saves
// whatever internal state it may hold, and gracefully terminates. An error is
// returned if Receiver cannot be gracefully shut down.
func (r *Receiver) Shutdown(context.Context) error {
	if r.cancel == nil {
		return nil // Already shut down.
	}

	r.cancel()
	r.wg.Wait()
	r.cancel = nil

	return nil
}

// Next performs a single polling cycle: one API call, one page of events. If
// pagination is in progress (a cursor exists in the checkpoint), it resumes
// from the cursor. Otherwise it starts a new interval [checkpoint, now).
//
// The checkpoint is persisted after each page with the current cursor, so a
// restart resumes precisely where it left off. When the API reports no more
// pages (NextCursor is empty), the interval is complete and the checkpoint
// advances to the upper bound of the interval.
//
// On error, the checkpoint is left at the last successfully processed position.
func (r *Receiver) Next(ctx context.Context) error {
	state := r.checkpoint.Get()

	if state.From.IsZero() { // First run.
		state.From = time.Now()
		if lookback := r.config.Lookback; lookback > 0 {
			state.From = state.From.Add(-lookback)
		}
	}

	if state.To.IsZero() { // New interval.
		state.To = time.Now()
		state.Cursor = ""
	}

	params := ListEventsParams{
		PageLimit: r.config.PageLimit,
		Filters:   r.config.Filters.ToFilters(),
		FromDate:  state.From,
		ToDate:    state.To,
	}
	if state.Cursor != "" {
		params.PageCursor = state.Cursor
	}

	resp, err := r.lister.ListEvents(ctx, params)
	if err != nil {
		return fmt.Errorf("listing events: %w", err)
	}

	if len(resp.Events) > 0 {
		for _, ev := range resp.Events {
			lr, err := ev.ToLogRecord()
			if err != nil {
				return fmt.Errorf("creating log record from event: %w", err)
			}
			r.logsBuilder.AppendLogRecord(lr)
		}

		logs := r.logsBuilder.Emit()
		if err := r.consumer.ConsumeLogs(ctx, logs); err != nil {
			return fmt.Errorf("consuming logs: %w", err)
		}
	}

	if resp.NextCursor == "" { // Pagination complete.
		state.From = state.To
		state.To = time.Time{}
		state.Cursor = ""
	} else {
		state.Cursor = resp.NextCursor
	}

	if err := r.checkpoint.Set(state); err != nil {
		return fmt.Errorf("saving checkpoint: %w", err)
	}

	return nil
}

// CreateReceiver uses component arguments to create and return a new Receiver in
// the component receiver.Logs interface.
func CreateReceiver(_ context.Context, settings receiver.Settings, config component.Config, logs consumer.Logs) (receiver.Logs, error) {
	conf, ok := config.(*Config)
	if !ok {
		return nil, errors.New("unsupported configuration type")
	}
	if err := conf.Validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	cp, err := NewCheckpointer(conf)
	if err != nil {
		return nil, fmt.Errorf("creating checkpoint store: %w", err)
	}

	return &Receiver{
		config:      conf,
		lister:      NewClient(conf.API.URL, conf.API.Key, WithTimeout(conf.API.Timeout)),
		consumer:    logs,
		checkpoint:  cp,
		logger:      settings.Logger,
		logsBuilder: metadata.NewLogsBuilder(settings),
	}, nil
}

// NewFactory can produce Receiver instances.
func NewFactory() receiver.Factory {
	return receiver.NewFactory(metadata.Type, CreateDefaultConfig, receiver.WithLogs(CreateReceiver, metadata.LogsStability))
}
