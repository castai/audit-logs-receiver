// Package checkpoint implements checkpoint storage for the CAST.AI audit logs
// receiver.
package checkpoint

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sync"
	"time"
)

// State tracks the receiver's position in the audit event stream.
type State struct {
	From   time.Time `json:"from"`
	To     time.Time `json:"to,omitempty"`
	Cursor string    `json:"cursor,omitempty"`
}

// Memory is a checkpoint store that keeps state in memory.
type Memory struct {
	mu    sync.Mutex
	state State
}

// NewMemory creates a new Memory checkpoint store.
func NewMemory() *Memory {
	return &Memory{}
}

// Get returns the latest State.
func (s *Memory) Get() State {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.state
}

// Set updates the State.
func (s *Memory) Set(state State) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.state = state
	return nil
}

// File is a checkpoint store that persists state to a file.
type File struct {
	mu       sync.Mutex
	state    State
	filename string
}

// NewFile creates a new File checkpoint store.
func NewFile(filename string) (*File, error) {
	s := &File{
		filename: filename,
	}

	data, err := os.ReadFile(filename)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return s, nil
		}

		return nil, fmt.Errorf("reading checkpoint file: %w", err)
	}

	if err := json.Unmarshal(data, &s.state); err != nil {
		return s, nil
	}

	return s, nil
}

// Get returns the latest State.
func (s *File) Get() State {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.state
}

// Set updates the State.
func (s *File) Set(state State) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := json.Marshal(state)
	if err != nil {
		return fmt.Errorf("marshaling checkpoint: %w", err)
	}
	if err = os.WriteFile(s.filename, data, 0600); err != nil {
		return err
	}

	s.state = state

	return nil
}
