package storage

import (
	"time"
)

type Storage interface {
	PutFromDate(time.Time)
	GetFromDate() time.Time
}

type inMemoryStorage struct {
	fromDate time.Time
}

func NewInMemoryStorage(fromDate time.Time) Storage {
	return &inMemoryStorage{
		fromDate: fromDate,
	}
}

func (s *inMemoryStorage) GetFromDate() time.Time {
	return s.fromDate
}

func (s *inMemoryStorage) PutFromDate(fromDate time.Time) {
	s.fromDate = fromDate
}

type persistentStorage struct {
	fromDate time.Time
}

func NewPersistentStorage(fromDate time.Time) Storage {
	return &persistentStorage{
		fromDate: fromDate,
	}
}

func (s *persistentStorage) GetFromDate() time.Time {
	return s.fromDate
}

func (s *persistentStorage) PutFromDate(fromDate time.Time) {
	s.fromDate = fromDate
}
