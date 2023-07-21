package storage

import (
	"github.com/samber/lo"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"testing"
	"time"
)

func TestInMemoryStorage(t *testing.T) {
	logger := zap.L()

	t.Run("when new poll data is created by a constructor then Get provides correct data", func(t *testing.T) {
		r := require.New(t)

		backFromNow := 99
		s := NewInMemoryStorage(logger, backFromNow)

		p := s.Get()
		r.True(p.CheckPoint.Before(time.Now().UTC()))
		r.True(p.CheckPoint.After(time.Now().UTC().Add(-time.Second*time.Duration(backFromNow) - 100*time.Millisecond)))
	})

	t.Run("when new poll data is set by calling Save method then Get provides correct data", func(t *testing.T) {
		r := require.New(t)

		s := NewInMemoryStorage(logger, 1)
		p := PollData{
			CheckPoint:     time.Now().UTC(),
			NextCheckPoint: lo.ToPtr(time.Now().UTC().Add(2 * time.Second)),
			ToDate:         lo.ToPtr(time.Now().UTC().Add(1 * time.Second)),
		}
		err := s.Save(p)
		r.NoError(err)
		r.Equal(p, s.Get())
	})

	t.Run("when new poll data is set by calling Save method then Get provides correct data", func(t *testing.T) {
		r := require.New(t)

		p := PollData{
			CheckPoint:     time.Now().UTC(),
			NextCheckPoint: lo.ToPtr(time.Now().UTC().Add(2 * time.Second)),
			ToDate:         lo.ToPtr(time.Now().UTC().Add(1 * time.Second)),
		}
		s := persistentStorage{
			inMemoryStorage: inMemoryStorage{
				logger:   nil,
				pollData: p,
			},
		}

		err := s.validate()
		r.NoError(err)
		r.Equal(p, s.Get())
	})
}

func TestPersistentStorageValidate(t *testing.T) {
	type fields struct {
		filename        string
		inMemoryStorage inMemoryStorage
	}
	tests := []struct {
		name    string
		fields  fields
		wantErr bool
	}{
		{
			name: "correct data without next_check_point and to_date",
			fields: fields{
				inMemoryStorage: inMemoryStorage{
					pollData: PollData{
						CheckPoint: time.Now().UTC(),
					},
				},
			},
			wantErr: false,
		},
		{
			name: "correct data with next_check_point and to_date",
			fields: fields{
				inMemoryStorage: inMemoryStorage{
					pollData: PollData{
						CheckPoint:     time.Now().UTC(),
						NextCheckPoint: lo.ToPtr(time.Now().UTC().Add(2 * time.Second)),
						ToDate:         lo.ToPtr(time.Now().UTC().Add(1 * time.Second)),
					},
				},
			},
			wantErr: false,
		},
		{
			name: "incorrect data: to_date is missing when next_check_point is provided",
			fields: fields{
				inMemoryStorage: inMemoryStorage{
					pollData: PollData{
						CheckPoint:     time.Now().UTC(),
						NextCheckPoint: lo.ToPtr(time.Now().UTC().Add(1 * time.Second)),
					},
				},
			},
			wantErr: true,
		},
		{
			name: "incorrect data: check_point is beyond next_check_point",
			fields: fields{
				inMemoryStorage: inMemoryStorage{
					pollData: PollData{
						CheckPoint:     time.Now().UTC().Add(3 * time.Second),
						NextCheckPoint: lo.ToPtr(time.Now().UTC().Add(2 * time.Second)),
						ToDate:         lo.ToPtr(time.Now().UTC().Add(1 * time.Second)),
					},
				},
			},
			wantErr: true,
		},
		{
			name: "incorrect data: check_point is beyond to_date",
			fields: fields{
				inMemoryStorage: inMemoryStorage{
					pollData: PollData{
						CheckPoint:     time.Now().UTC().Add(2 * time.Second),
						NextCheckPoint: lo.ToPtr(time.Now().UTC().Add(3 * time.Second)),
						ToDate:         lo.ToPtr(time.Now().UTC().Add(1 * time.Second)),
					},
				},
			},
			wantErr: true,
		},
		{
			name: "incorrect data: to_date is beyond next_check_point",
			fields: fields{
				inMemoryStorage: inMemoryStorage{
					pollData: PollData{
						CheckPoint:     time.Now().UTC(),
						NextCheckPoint: lo.ToPtr(time.Now().UTC().Add(1 * time.Second)),
						ToDate:         lo.ToPtr(time.Now().UTC().Add(2 * time.Second)),
					},
				},
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &persistentStorage{
				filename:        tt.fields.filename,
				inMemoryStorage: tt.fields.inMemoryStorage,
			}
			if err := s.validate(); (err != nil) != tt.wantErr {
				t.Errorf("validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
