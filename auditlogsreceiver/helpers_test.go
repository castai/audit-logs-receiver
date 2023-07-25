package auditlogs

import (
	"context"
	"github.com/castai/otel-receivers/audit-logs/storage"
	"github.com/jarcoal/httpmock"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/plog"
	"net/http"
	"strconv"
	"testing"
	"time"
)

// This struct complying to an Open Telemetry consumer.Logs interface so rolling out a mock manually
// instead of generated one.
type logsConsumerMock struct {
	ConsumeLogsFunc func(ld plog.Logs) error
}

func (a logsConsumerMock) Capabilities() consumer.Capabilities {
	panic("implement me")
}

func (a logsConsumerMock) ConsumeLogs(_ context.Context, ld plog.Logs) error {
	return a.ConsumeLogsFunc(ld)
}

func newResponseWithOneItem(lastLogTimestamp time.Time) string {
	return `{
    "items": [
        {
            "id": "824e7a47-b8e3-430e-8a7d-e9db83781e6e",
            "eventType": "clusterDeleted",
            "initiatedBy": {
                "id": "google-oauth2|100187903622338083673",
                "name": "Andrej Kislovskij",
                "email": "andrej@cast.ai"
            },
            "time": "` + lastLogTimestamp.UTC().Format(timestampLayout) + `",
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
}`
}

func newResponseWithTwoItem(lastLogTimestamp time.Time, cursorData string) string {
	return `{
    "items": [
        {
            "id": "824e7a47-b8e3-430e-8a7d-e9db83781e6e",
            "eventType": "clusterDeleted",
            "initiatedBy": {
                "id": "google-oauth2|100187903622338083673",
                "name": "Andrej Kislovskij",
                "email": "andrej@cast.ai"
            },
            "time": "` + lastLogTimestamp.Add(-1*time.Millisecond).UTC().Format(timestampLayout) + `",
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
        },
        {
            "id": "824e7a47-b8e3-430e-8a7d-e9db83781e6e",
            "eventType": "clusterDeleted",
            "initiatedBy": {
                "id": "google-oauth2|100187903622338083673",
                "name": "Andrej Kislovskij",
                "email": "andrej@cast.ai"
            },
            "time": "` + lastLogTimestamp.UTC().Format(timestampLayout) + `",
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
	],
	"nextCursor": "` + cursorData + `"
}`
}

func defaultResponderWithAssertions(t *testing.T, data *storage.PollData, pageLimit int, body string) httpmock.Responder {
	r := require.New(t)

	return func(req *http.Request) (*http.Response, error) {
		queryValues := req.URL.Query()
		r.Equal(3, len(queryValues))

		// Audit Logs API accepts timestamps in UTC.
		fromDate, err := time.ParseInLocation(timestampLayout, queryValues["fromDate"][0], time.UTC)
		r.NoError(err)
		r.WithinDuration(data.CheckPoint, fromDate, 0)

		toDate, err := time.ParseInLocation(timestampLayout, queryValues["toDate"][0], time.UTC)
		r.NoError(err)
		r.WithinDuration(*data.ToDate, toDate, 0)

		r.Equal(strconv.Itoa(pageLimit), queryValues["page.limit"][0])

		return httpmock.NewStringResponse(200, body), nil
	}
}
