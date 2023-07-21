package storage

import (
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/samber/lo"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestPersistentStorage(t *testing.T) {
	logger := zap.L()

	t.Run("when new poll data is created by a constructor then Get provides correct data", func(t *testing.T) {
		r := require.New(t)

		p := PollData{
			CheckPoint:     time.Now().UTC(),
			NextCheckPoint: lo.ToPtr(time.Now().UTC().Add(2 * time.Second)),
			ToDate:         lo.ToPtr(time.Now().UTC().Add(1 * time.Second)),
		}
		jsonBytes, err := json.Marshal(&p)
		r.NoError(err)

		filename := uuid.NewString() + ".json"
		err = os.WriteFile(filename, jsonBytes, os.ModePerm)
		r.NoError(err)
		defer func() {
			err = os.Remove(filename)
			r.NoError(err)
		}()

		s, err := NewPersistentStorage(logger, filename)
		r.NoError(err)
		r.Equal(p, s.Get())
	})

	t.Run("when no file is present then a new one is created and Get provides correct data", func(t *testing.T) {
		r := require.New(t)

		filename := uuid.NewString() + ".json"
		s, err := NewPersistentStorage(logger, filename)
		r.NoError(err)
		defer func() {
			os.Remove(filename)
		}()

		p := s.Get()
		r.True(p.CheckPoint.Before(time.Now().UTC()))
		r.True(p.CheckPoint.After(time.Now().UTC().Add(-100 * time.Millisecond)))

		jsonFile, err := os.Open(filename)
		r.NoError(err)
		defer jsonFile.Close()
	})
}
