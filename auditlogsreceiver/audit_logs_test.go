package auditlogs

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"testing"
	"time"

	"github.com/castai/otel-receivers/audit-logs/storage"
	mock_storage "github.com/castai/otel-receivers/audit-logs/storage/mock"
	"github.com/golang/mock/gomock"
	"github.com/google/uuid"
	"github.com/jarcoal/httpmock"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.uber.org/zap"
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
		httpmock.RegisterResponder(http.MethodGet, `=~^https:\/\/api\.cast\.ai/v1/audit.?`, func(req *http.Request) (*http.Response, error) {
			queryValues := req.URL.Query()
			r.Equal(3, len(queryValues))

			fmt.Printf("--> HERE 1 %v\n", req.URL.Query())

			// Audit Logs API accepts timestamps in UTC.
			dd, err := time.ParseInLocation(timestampLayout, queryValues["fromDate"][0], time.UTC)
			r.NoError(err)
			r.WithinDuration(data.CheckPoint, dd, 0)

			ee, err := time.ParseInLocation(timestampLayout, queryValues["toDate"][0], time.UTC)
			r.NoError(err)
			r.WithinDuration(*data.ToDate, ee, 0)

			r.Equal(strconv.Itoa(restConfig.PageLimit), queryValues["page.limit"][0])

			return httpmock.NewStringResponse(200, `{}`), nil
		})

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
			consumeLogsFunc: func(logs plog.Logs) error {
				r.Positive(allowedCalls)
				allowedCalls--

				// TODO: Audit Logs -> Open Telemetry logs mapping should unit tested separately.
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
		httpmock.RegisterResponder(http.MethodGet, `=~^https:\/\/api\.cast\.ai/v1/audit.?`, func(req *http.Request) (*http.Response, error) {
			queryValues := req.URL.Query()
			r.Equal(3, len(queryValues))

			// Audit Logs API accepts timestamps in UTC.
			dd, err := time.ParseInLocation(timestampLayout, queryValues["fromDate"][0], time.UTC)
			r.NoError(err)
			r.WithinDuration(data.CheckPoint, dd, 0)

			ee, err := time.ParseInLocation(timestampLayout, queryValues["toDate"][0], time.UTC)
			r.NoError(err)
			r.WithinDuration(*data.ToDate, ee, 0)

			r.Equal(strconv.Itoa(restConfig.PageLimit), queryValues["page.limit"][0])

			return httpmock.NewStringResponse(200, `{
    "items": [
        {
            "id": "824e7a47-b8e3-430e-8a7d-e9db83781e6e",
            "eventType": "clusterDeleted",
            "initiatedBy": {
                "id": "google-oauth2|100187903622338083673",
                "name": "Andrej Kislovskij",
                "email": "andrej@cast.ai"
            },
            "time": "`+lastLogTimestamp.UTC().Format(timestampLayout)+`",
            "event": {
                "cluster": {
                    "cloudCredentialsIDs": "b72c816f-5b46-4aa2-b832-a834e0a75e30",
                    "id": "1e6e37e0-7a06-4fde-8eb0-019ae8b1cf4f",
                    "name": "andrej-cluster-07-13-1",
                    "providerType": "gke",
                    "region": "europe-west1"
                }
            },
            "labels": {
                "clusterId": "1e6e37e0-7a06-4fde-8eb0-019ae8b1cf4f"
            }
        }
	]
}`), nil
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
}

// This struct complying to an external consumer.Logs interface so rolling out a mock manually instead of generated one.
type logsConsumerMock struct {
	consumeLogsFunc func(ld plog.Logs) error
}

func (a logsConsumerMock) Capabilities() consumer.Capabilities {
	panic("implement me")
}

func (a logsConsumerMock) ConsumeLogs(_ context.Context, ld plog.Logs) error {
	return a.consumeLogsFunc(ld)
}
