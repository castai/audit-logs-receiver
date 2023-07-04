package auditlogs

import (
	"context"
	"fmt"
	"sync"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.uber.org/zap"
)

type auditLogsReceiver struct {
	logger       *zap.Logger
	pollInterval time.Duration

	nextStartTime time.Time
	wg            *sync.WaitGroup
	doneChan      chan bool
}

func newAuditLogsReceiver(cfg *Config, logger *zap.Logger, consumer consumer.Logs) *auditLogsReceiver {
	return &auditLogsReceiver{
		logger:        logger,
		pollInterval:  time.Second * time.Duration(cfg.PollIntervalSec),
		nextStartTime: time.Now().Add(time.Duration(cfg.PollIntervalSec)),
		wg:            &sync.WaitGroup{},
		doneChan:      make(chan bool),
	}
}

func (a *auditLogsReceiver) Start(ctx context.Context, host component.Host) error {
	fmt.Printf("--> HERE 1.1\n")

	a.logger.Debug("starting audit logs receiver")
	a.wg.Add(1)
	go a.startPolling(ctx)

	return nil
}

func (a *auditLogsReceiver) Shutdown(ctx context.Context) error {
	fmt.Printf("--> HERE 1.3\n")

	a.logger.Debug("shutting down audit logs receiver")
	close(a.doneChan)
	a.wg.Wait()

	return nil
}

func (a *auditLogsReceiver) startPolling(ctx context.Context) {
	defer a.wg.Done()

	t := time.NewTicker(a.pollInterval)
	defer t.Stop()

	for {
		fmt.Printf("--> HERE 1.2 %v\n", time.Now())

		//if l.autodiscover != nil {
		//	group, err := l.discoverGroups(ctx, l.autodiscover)
		//	if err != nil {
		//		l.logger.Error("unable to perform discovery of log groups", zap.Error(err))
		//		continue
		//	}
		//	l.groupRequests = group
		//}
		//
		//err := l.poll(ctx)
		//if err != nil {
		//	l.logger.Error("there was an error during the poll", zap.Error(err))
		//}

		select {
		case <-ctx.Done():
			return
		case <-a.doneChan:
			return
		case <-t.C:
			continue
		}
	}
}
