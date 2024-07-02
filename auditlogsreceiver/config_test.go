package auditlogsreceiver

import (
	"github.com/google/uuid"
	"testing"
)

func TestConfigValidate(t *testing.T) {
	type fields struct {
		API             API
		PollIntervalSec int
		PageLimit       int
		Storage         map[string]interface{}
	}
	tests := []struct {
		name    string
		fields  fields
		wantErr bool
	}{
		{
			name: "in-memory storage correct data",
			fields: fields{
				API: API{
					Url: "https://api.cast.ai",
					Key: uuid.NewString(),
				},
				PollIntervalSec: 10,
				PageLimit:       100,
				Storage: map[string]interface{}{
					"type":              "in-memory",
					"back_from_now_sec": 10,
				},
			},
			wantErr: false,
		},
		{
			name: "persistent storage correct data",
			fields: fields{
				API: API{
					Url: "https://api.cast.ai",
					Key: uuid.NewString(),
				},
				PollIntervalSec: 10,
				PageLimit:       100,
				Storage: map[string]interface{}{
					"type":     "persistent",
					"filename": uuid.NewString() + ".json",
				},
			},
			wantErr: false,
		},
		{
			name: "missing API URL",
			fields: fields{
				API: API{
					Url: "",
					Key: uuid.NewString(),
				},
				PollIntervalSec: 10,
				PageLimit:       100,
				Storage: map[string]interface{}{
					"type": "persistent",
				},
			},
			wantErr: true,
		},
		{
			name: "missing API Key",
			fields: fields{
				API: API{
					Url: "https://api.cast.ai",
					Key: "",
				},
				PollIntervalSec: 10,
				PageLimit:       100,
				Storage: map[string]interface{}{
					"type": "persistent",
				},
			},
			wantErr: true,
		},
		{
			name: "poll interval outside of valid ranges",
			fields: fields{
				API: API{
					Url: "https://api.cast.ai",
					Key: uuid.NewString(),
				},
				PollIntervalSec: 0,
				PageLimit:       100,
				Storage: map[string]interface{}{
					"type": "persistent",
				},
			},
			wantErr: true,
		},
		{
			name: "page limit is outside of valid ranges",
			fields: fields{
				API: API{
					Url: "https://api.cast.ai",
					Key: uuid.NewString(),
				},
				PollIntervalSec: 10,
				PageLimit:       1001,
				Storage: map[string]interface{}{
					"type": "persistent",
				},
			},
			wantErr: true,
		},
		{
			name: "invalid storage type",
			fields: fields{
				API: API{
					Url: "https://api.cast.ai",
					Key: uuid.NewString(),
				},
				PollIntervalSec: 10,
				PageLimit:       100,
				Storage: map[string]interface{}{
					"type": "invalid type",
				},
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := Config{
				API:             tt.fields.API,
				PollIntervalSec: tt.fields.PollIntervalSec,
				PageLimit:       tt.fields.PageLimit,
				Storage:         tt.fields.Storage,
			}
			if err := c.Validate(); (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
