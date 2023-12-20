package auditlogs

import (
	"context"
	"net/http"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/google/uuid"
	"github.com/jarcoal/httpmock"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.uber.org/zap"

	"github.com/castai/audit-logs-receiver/audit-logs/storage"
	mock_storage "github.com/castai/audit-logs-receiver/audit-logs/storage/mock"
)

func TestPoll(t *testing.T) {
	logger := zap.L()

	t.Run("when polling audit logs using correct parameters and no data is present then an empty response is processed correctly", func(t *testing.T) {
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

		expectedClusterID := uuid.NewString()

		// Polling parameters are not known at the moment of registering a responder, so asserting params in the responder vs using an exact query.
		httpmock.RegisterResponder(
			http.MethodGet,
			`=~^https:\/\/api\.cast\.ai/v1/audit.?`,
			defaultResponderWithAssertions(t, &data, restConfig.PageLimit, `{}`, expectedClusterID))

		receiver := auditLogsReceiver{
			logger:    logger,
			pageLimit: restConfig.PageLimit,
			filter: filters{
				clusterID: &expectedClusterID,
			},
			storage: storageMock,
			rest:    rest,
		}
		err := receiver.poll(ctx, nil)
		r.NoError(err)
	})

	t.Run("when polling audit logs using correct parameters and few items is present then a response is processed correctly", func(t *testing.T) {
		r := require.New(t)
		ctx := context.Background()
		checkPointTimestamp := time.Now().Add(-10 * time.Second)
		lastLogTimestamp := time.Now().Add(-9 * time.Second)

		mockCtrl := gomock.NewController(t)
		defer mockCtrl.Finish()

		storageMock := mock_storage.NewMockStorage(mockCtrl)
		var nextCheckPoint *time.Time
		var data storage.PollData
		gomock.InOrder(
			storageMock.EXPECT().
				Get().
				Return(storage.PollData{
					CheckPoint: checkPointTimestamp,
				}),
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
				}),
			storageMock.EXPECT().
				Save(gomock.Any()).
				Do(func(dt storage.PollData) {
					r.WithinDuration(checkPointTimestamp, dt.CheckPoint, 0)
					r.WithinDuration(lastLogTimestamp, *dt.ToDate, 0)
					r.WithinDuration(*nextCheckPoint, *dt.NextCheckPoint, 0)
				}),
			storageMock.EXPECT().
				Save(gomock.Any()).
				Do(func(dt storage.PollData) {
					r.WithinDuration(*nextCheckPoint, dt.CheckPoint, 0)
					r.Empty(dt.ToDate)
					r.Empty(dt.NextCheckPoint)
				}),
		)

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

		expectedClusterID := uuid.NewString()

		// Polling parameters are not known at the moment of registering a responder, so asserting params in the responder vs using an exact query.
		httpmock.RegisterResponder(
			http.MethodGet,
			`=~^https:\/\/api\.cast\.ai/v1/audit.?`,
			defaultResponderWithAssertions(t, &data, restConfig.PageLimit, newResponseWithOneItem(lastLogTimestamp), expectedClusterID))

		receiver := auditLogsReceiver{
			logger:    logger,
			pageLimit: restConfig.PageLimit,
			filter: filters{
				clusterID: &expectedClusterID,
			},
			storage:  storageMock,
			rest:     rest,
			consumer: consumerMock,
		}
		err := receiver.poll(ctx, nil)
		r.NoError(err)
	})

	t.Run("when polling audit logs using correct parameters and pagination is involved then a response is processed correctly", func(t *testing.T) {
		r := require.New(t)
		ctx := context.Background()
		checkPointTimestamp := time.Now().Add(-10 * time.Second)
		firstPageLastLogTimestamp := time.Now().Add(-7 * time.Second)
		secondPageLastLogTimestamp := time.Now().Add(-9 * time.Second)
		cursorData := uuid.NewString()

		mockCtrl := gomock.NewController(t)
		defer mockCtrl.Finish()

		storageMock := mock_storage.NewMockStorage(mockCtrl)
		var nextCheckPoint *time.Time
		var data storage.PollData
		gomock.InOrder(
			storageMock.EXPECT().
				Get().
				Return(storage.PollData{
					CheckPoint: checkPointTimestamp,
				}),
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
				}),
			storageMock.EXPECT().
				Save(gomock.Any()).
				Do(func(dt storage.PollData) {
					r.WithinDuration(checkPointTimestamp, dt.CheckPoint, 0)
					r.WithinDuration(firstPageLastLogTimestamp, *dt.ToDate, 0)
					r.WithinDuration(*nextCheckPoint, *dt.NextCheckPoint, 0)
				}),
			storageMock.EXPECT().
				Save(gomock.Any()).
				Do(func(dt storage.PollData) {
					r.WithinDuration(checkPointTimestamp, dt.CheckPoint, 0)
					r.WithinDuration(secondPageLastLogTimestamp, *dt.ToDate, 0)
					r.WithinDuration(*nextCheckPoint, *dt.NextCheckPoint, 0)
				}),
			storageMock.EXPECT().
				Save(gomock.Any()).
				Do(func(dt storage.PollData) {
					r.WithinDuration(*nextCheckPoint, dt.CheckPoint, 0)
					r.Empty(dt.ToDate)
					r.Empty(dt.NextCheckPoint)
				}),
		)

		responderCalls := 0
		consumerMock := logsConsumerMock{
			ConsumeLogsFunc: func(logs plog.Logs) error {
				switch responderCalls {
				case 1:
					// 2 log entries are returned after the first API call.
					r.Equal(2, logs.LogRecordCount())
				case 2:
					// 1 log entries are returned after the second API call.
					r.Equal(1, logs.LogRecordCount())
				default:
					r.Fail("audit logs API was called too many times")
				}

				return nil
			},
		}

		restConfig := Config{
			API: API{
				Url: "https://api.cast.ai",
				Key: uuid.NewString(),
			},
			PageLimit: 2,
		}
		rest := newRestyClient(&restConfig)
		httpmock.ActivateNonDefault(rest.GetClient())
		defer httpmock.Reset()

		// Polling parameters are not known at the moment of registering a responder, so asserting params in the responder vs using an exact query.
		httpmock.RegisterResponder(
			http.MethodGet,
			`=~^https:\/\/api\.cast\.ai/v1/audit.?`,
			func(req *http.Request) (*http.Response, error) {
				responderCalls++
				queryValues := req.URL.Query()

				// Mocking pagination
				var body string
				switch responderCalls {
				case 1:
					r.Equal(3, len(queryValues))

					// Audit Logs API accepts timestamps in UTC.
					fromDate, err := time.ParseInLocation(timestampLayout, queryValues["fromDate"][0], time.UTC)
					r.NoError(err)
					r.WithinDuration(data.CheckPoint, fromDate, 0)

					toDate, err := time.ParseInLocation(timestampLayout, queryValues["toDate"][0], time.UTC)
					r.NoError(err)
					r.WithinDuration(*data.ToDate, toDate, 0)

					r.Equal(strconv.Itoa(restConfig.PageLimit), queryValues["page.limit"][0])

					body = newResponseWithTwoItem(firstPageLastLogTimestamp, cursorData)
				case 2:
					r.Equal(2, len(queryValues))

					r.Equal(cursorData, queryValues["page.cursor"][0])

					r.Equal(strconv.Itoa(restConfig.PageLimit), queryValues["page.limit"][0])

					body = newResponseWithOneItem(secondPageLastLogTimestamp)
				default:
					r.Fail("audit logs API was called too many times")
				}
				return httpmock.NewStringResponse(200, body), nil
			})

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

	t.Run("should cancel work immediately after shutdown is called", func(t *testing.T) {
		r := require.New(t)
		ctx := context.Background()

		mockCtrl := gomock.NewController(t)
		defer mockCtrl.Finish()

		storageMock := mock_storage.NewMockStorage(mockCtrl)
		storageMock.EXPECT().
			Get().
			Return(storage.PollData{
				CheckPoint: time.Now(),
			}).AnyTimes()
		storageMock.EXPECT().
			Save(gomock.Any()).AnyTimes()

		consumerMock := logsConsumerMock{
			ConsumeLogsFunc: func(logs plog.Logs) error {
				return nil
			},
		}

		restConfig := Config{
			API: API{
				Url: "https://api.cast.ai",
				Key: uuid.NewString(),
			},
			PageLimit: 2,
		}
		rest := newRestyClient(&restConfig)
		httpmock.ActivateNonDefault(rest.GetClient())
		defer httpmock.Reset()

		reqStarted := make(chan struct{})
		reqStoped := make(chan struct{})
		httpmock.RegisterResponder(
			http.MethodGet,
			`=~^https:\/\/api\.cast\.ai/v1/audit.?`,
			func(req *http.Request) (*http.Response, error) {
				close(reqStarted)
				<-req.Context().Done()
				close(reqStoped)
				return httpmock.NewStringResponse(200, "none"), nil
			})

		receiver := auditLogsReceiver{
			logger:       logger,
			pageLimit:    restConfig.PageLimit,
			pollInterval: 1 * time.Millisecond,
			wg:           &sync.WaitGroup{},
			storage:      storageMock,
			rest:         rest,
			consumer:     consumerMock,
		}
		err := receiver.Start(ctx, nil)
		<-reqStarted
		go func() {
			err := receiver.Shutdown(ctx)
			r.NoError(err)
		}()
		<-reqStoped
		r.NoError(err)
	})
}
