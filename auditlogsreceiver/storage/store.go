package storage

import (
	"time"
)

type Store interface {
	PutFromDate(time.Time)
	GetFromDate() time.Time
}

type ephemeralStore struct {
	fromDate time.Time
}

func NewEphemeralStore(fromDate time.Time) Store {
	return &ephemeralStore{
		fromDate: fromDate,
	}
}

func (s *ephemeralStore) GetFromDate() time.Time {
	return s.fromDate
}

func (s *ephemeralStore) PutFromDate(fromDate time.Time) {
	s.fromDate = fromDate
}
