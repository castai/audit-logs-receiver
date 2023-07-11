package storage

import (
	"fmt"
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
	fmt.Printf("--> HERE 4.1 %v\n", s.fromDate)

	return s.fromDate
}

func (s *ephemeralStore) PutFromDate(fromDate time.Time) {
	fmt.Printf("--> HERE 4.2 %v\n", fromDate)

	s.fromDate = fromDate
}
