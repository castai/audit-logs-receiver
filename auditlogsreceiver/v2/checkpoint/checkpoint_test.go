package checkpoint_test

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/castai/audit-logs-receiver/audit-logs/v2/checkpoint"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMemoryGetSet(t *testing.T) {
	s := checkpoint.NewMemory()

	state := s.Get()
	assert.True(t, state.From.IsZero())

	now := time.Now()
	require.NoError(t, s.Set(checkpoint.State{From: now}))

	state = s.Get()
	assert.EqualValues(t, now.UnixNano(), state.From.UnixNano())
}

func TestFileGetSet(t *testing.T) {
	filename := filepath.Join(t.TempDir(), "checkpoint.json")
	s, err := checkpoint.NewFile(filename)
	require.NoError(t, err)

	state := s.Get()
	assert.True(t, state.From.IsZero())

	now := time.Now()
	require.NoError(t, s.Set(checkpoint.State{From: now}))

	state = s.Get()
	assert.EqualValues(t, now.UnixNano(), state.From.UnixNano())

	s2, err := checkpoint.NewFile(filename)
	require.NoError(t, err)

	state = s2.Get()
	assert.EqualValues(t, now.UnixNano(), state.From.UnixNano())
}

func TestFileNotExists(t *testing.T) {
	s, err := checkpoint.NewFile(filepath.Join(t.TempDir(), "nonexistent.json"))
	require.NoError(t, err)

	state := s.Get()
	assert.True(t, state.From.IsZero())
}

func TestFileCorruptionRecovery(t *testing.T) {
	filename := filepath.Join(t.TempDir(), "checkpoint.json")
	require.NoError(t, os.WriteFile(filename, []byte("not valid json"), 0644))

	// A corrupted checkpoint must surface as an error, not silently restart
	// from the beginning (which could skip undelivered events).
	_, err := checkpoint.NewFile(filename)
	require.Error(t, err)
	require.ErrorContains(t, err, "parsing checkpoint file")

	// Once the operator removes or fixes the file, the store works again.
	require.NoError(t, os.Remove(filename))

	var now time.Time
	{
		s, err := checkpoint.NewFile(filename)
		require.NoError(t, err)
		now = time.Now()
		require.NoError(t, s.Set(checkpoint.State{From: now}))
	}

	s2, err := checkpoint.NewFile(filename)
	require.NoError(t, err)
	state := s2.Get()
	assert.EqualValues(t, now.UnixNano(), state.From.UnixNano())
}

func TestFileConcurrentAccess(t *testing.T) {
	s, err := checkpoint.NewFile(filepath.Join(t.TempDir(), "checkpoint.json"))
	require.NoError(t, err)

	var wg sync.WaitGroup
	for n := range 100 {
		wg.Go(func() {
			require.NoError(t, s.Set(checkpoint.State{From: time.Now().Add(time.Duration(n) * time.Second)}))
			require.NotEmpty(t, s.Get())
		})
	}
	wg.Wait()
}
