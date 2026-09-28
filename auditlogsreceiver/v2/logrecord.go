package auditlogsreceiver

import (
	"encoding/base64"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
)

// ToLogRecord creates and returns an OpenTelemetry log record from the Event.
func (e *Event) ToLogRecord() (plog.LogRecord, error) {
	r := plog.NewLogRecord()

	r.SetSeverityNumber(plog.SeverityNumber(e.EventSeverity))
	r.SetSeverityText(e.EventSeverityText)
	r.Body().SetStr(e.Description)
	r.SetTimestamp(pcommon.NewTimestampFromTime(e.OccurredAt))
	r.SetObservedTimestamp(pcommon.NewTimestampFromTime(time.Now()))

	attr := map[string]any{
		"tenant.id":      e.TenantID,
		"event.id":       e.EventID,
		"event.domain":   e.EventDomain,
		"event.resource": e.EventResource,
		"event.action":   e.EventAction,
		"ingested_at":    e.IngestedAt.UTC().Format(time.RFC3339Nano),
	}
	if e.Body != "" {
		attr["event.body"] = decodeEventBody(e.Body)
	}
	if e.ClusterID != "" {
		attr["cluster.id"] = e.ClusterID
	}
	if e.RequestID != "" {
		attr["request.id"] = e.RequestID
	}
	if e.CorrelationID != "" {
		attr["correlation.id"] = e.CorrelationID
		attr["correlation.count"] = e.CorrelatedEventCount

		var traceID pcommon.TraceID
		if correlationID, err := uuid.Parse(e.CorrelationID); err == nil {
			copy(traceID[:], correlationID[:])
			r.SetTraceID(traceID)
		}
	}
	if e.Actor != nil {
		attr["actor.id"] = e.Actor.ID
		attr["actor.type"] = e.Actor.Type
		attr["actor.display_name"] = e.Actor.DisplayName
		attr["actor.email"] = e.Actor.Email
	}
	if e.Resource != nil {
		attr["resource.type"] = e.Resource.Type
		attr["resource.id"] = e.Resource.ID
		attr["resource.display_name"] = e.Resource.DisplayName
	}
	if len(e.Labels) > 0 {
		attr["labels"] = e.Labels
	}

	if err := r.Attributes().FromRaw(attr); err != nil {
		return plog.LogRecord{}, err
	}

	return r, nil
}

// decodeEventBody converts the API's event body into a structured map.
func decodeEventBody(body string) any {
	if decoded, err := base64.StdEncoding.DecodeString(body); err == nil {
		var parsed map[string]any
		if err := json.Unmarshal(decoded, &parsed); err == nil {
			return parsed
		}
		return string(decoded) // When valid base64 but not JSON, return decoded text.
	}

	// Future-proof: if the proto changes to string, the body is plain JSON.
	var parsed map[string]any
	if err := json.Unmarshal([]byte(body), &parsed); err == nil {
		return parsed
	}

	return body // Not base64, not JSON — return the raw string.
}
