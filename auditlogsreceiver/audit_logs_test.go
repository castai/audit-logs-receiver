package auditlogs

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/google/uuid"
	"github.com/jarcoal/httpmock"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.uber.org/zap"

	"github.com/castai/otel-receivers/audit-logs/storage"
	mock_storage "github.com/castai/otel-receivers/audit-logs/storage/mock"
)

func TestPoll(t *testing.T) {
	logger := zap.L()

	t.Run("when polling audit logs using correct parameters and no data is present then an empty response is returned", func(t *testing.T) {
		r := require.New(t)
		ctx := context.Background()
		checkPointTimestamp := time.Now().Add(-10 * time.Second)

		mockCtrl := gomock.NewController(t)
		defer mockCtrl.Finish()

		storageMock := mock_storage.NewMockStorage(mockCtrl)
		storageMock.EXPECT().
			Get().
			Return(storage.PollData{
				CheckPoint: checkPointTimestamp,
			})
		var nextCheckPoint *time.Time
		var data storage.PollData
		storageMock.EXPECT().
			Save(gomock.Any()).
			Do(func(dt storage.PollData) {
				r.WithinDuration(checkPointTimestamp, dt.CheckPoint, 0)
				r.WithinDuration(time.Now(), *dt.ToDate, time.Millisecond)
				r.WithinDuration(*dt.NextCheckPoint, *dt.ToDate, 0)

				// Used to asset the second call to Save method.
				nextCheckPoint = dt.NextCheckPoint

				// Used to assert query params.
				data = dt
			})
		storageMock.EXPECT().
			Save(gomock.Any()).
			Do(func(dt storage.PollData) {
				r.WithinDuration(*nextCheckPoint, dt.CheckPoint, 0)
				r.Empty(dt.ToDate)
				r.Empty(dt.NextCheckPoint)
			})

		restConfig := Config{
			API: API{
				Url: "https://api.cast.ai",
				Key: uuid.NewString(),
			},
			PageLimit: 11,
		}
		rest := newRestyClient(&restConfig)
		httpmock.ActivateNonDefault(rest.GetClient())
		defer httpmock.Reset()

		// Polling parameters are not known at the moment of registering a responder, so asserting params in the responder vs using an exact query.
		httpmock.RegisterResponder(
			http.MethodGet,
			`=~^https:\/\/api\.cast\.ai/v1/audit.?`,
			defaultResponderWithAssertions(t, &data, restConfig.PageLimit, `{}`))

		receiver := auditLogsReceiver{
			logger:    logger,
			pageLimit: restConfig.PageLimit,
			storage:   storageMock,
			rest:      rest,
		}
		err := receiver.poll(ctx, nil)
		r.NoError(err)
	})

	t.Run("when polling audit logs using correct parameters and few items is present then a correct response is returned", func(t *testing.T) {
		r := require.New(t)
		ctx := context.Background()
		checkPointTimestamp := time.Now().Add(-10 * time.Second)
		lastLogTimestamp := time.Now().Add(-9 * time.Second)

		mockCtrl := gomock.NewController(t)
		defer mockCtrl.Finish()

		storageMock := mock_storage.NewMockStorage(mockCtrl)
		storageMock.EXPECT().
			Get().
			Return(storage.PollData{
				CheckPoint: checkPointTimestamp,
			})
		var nextCheckPoint *time.Time
		var data storage.PollData
		storageMock.EXPECT().
			Save(gomock.Any()).
			Do(func(dt storage.PollData) {
				r.WithinDuration(checkPointTimestamp, dt.CheckPoint, 0)
				r.WithinDuration(time.Now(), *dt.ToDate, time.Millisecond)
				r.WithinDuration(*dt.NextCheckPoint, *dt.ToDate, 0)

				// Used to asset the second call to Save method.
				nextCheckPoint = dt.NextCheckPoint

				// Used to assert query params.
				data = dt
			})
		storageMock.EXPECT().
			Save(gomock.Any()).
			Do(func(dt storage.PollData) {
				r.WithinDuration(checkPointTimestamp, dt.CheckPoint, 0)
				r.WithinDuration(lastLogTimestamp, *dt.ToDate, 0)
				r.WithinDuration(*nextCheckPoint, *dt.NextCheckPoint, 0)
			})
		storageMock.EXPECT().
			Save(gomock.Any()).
			Do(func(dt storage.PollData) {
				r.WithinDuration(*nextCheckPoint, dt.CheckPoint, 0)
				r.Empty(dt.ToDate)
				r.Empty(dt.NextCheckPoint)
			})

		allowedCalls := 1
		consumerMock := logsConsumerMock{
			ConsumeLogsFunc: func(logs plog.Logs) error {
				r.Positive(allowedCalls)
				allowedCalls--

				// TODO: Audit Logs -> Open Telemetry logs mapping should be unit tested separately.
				r.Equal(1, logs.LogRecordCount())

				return nil
			},
		}

		restConfig := Config{
			API: API{
				Url: "https://api.cast.ai",
				Key: uuid.NewString(),
			},
			PageLimit: 11,
		}
		rest := newRestyClient(&restConfig)
		httpmock.ActivateNonDefault(rest.GetClient())
		defer httpmock.Reset()

		// Polling parameters are not known at the moment of registering a responder, so asserting params in the responder vs using an exact query.
		httpmock.RegisterResponder(
			http.MethodGet,
			`=~^https:\/\/api\.cast\.ai/v1/audit.?`,
			defaultResponderWithAssertions(t, &data, restConfig.PageLimit, newResponseBody(lastLogTimestamp)))

		receiver := auditLogsReceiver{
			logger:    logger,
			pageLimit: restConfig.PageLimit,
			storage:   storageMock,
			rest:      rest,
			consumer:  consumerMock,
		}
		err := receiver.poll(ctx, nil)
		r.NoError(err)
	})
}
