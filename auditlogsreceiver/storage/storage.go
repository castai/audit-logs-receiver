//go:generate mockgen -source $GOFILE -destination mock/$GOFILE . Storage
package storage

import (
	"encoding/json"
	"errors"
	"fmt"
	"go.uber.org/zap"
	"io"
	"os"
	"time"
)

type PollData struct {
	CheckPoint     time.Time  `json:"check_point"`
	NextCheckPoint *time.Time `json:"next_check_point,omitempty"`
	ToDate         *time.Time `json:"to_date,omitempty"`
}

type Storage interface {
	Get() PollData
	Save(PollData) error
}

type inMemoryStorage struct {
	logger   *zap.Logger
	pollData PollData
}

func NewInMemoryStorage(logger *zap.Logger, backFromNowSec int) Storage {
	logger.Info("new in-memory storage was created", zap.Int("back_from_now_sec", backFromNowSec))

	return &inMemoryStorage{
		logger: logger,
		pollData: PollData{
			CheckPoint:     time.Now().Add(-time.Second * time.Duration(backFromNowSec)),
			NextCheckPoint: nil,
			ToDate:         nil,
		},
	}
}

func (s *inMemoryStorage) Get() PollData {
	return s.pollData
}

func (s *inMemoryStorage) Save(data PollData) error {
	s.pollData = data
	return nil
}

type persistentStorage struct {
	filename string
	inMemoryStorage
}

func NewPersistentStorage(logger *zap.Logger, filename string) (Storage, error) {
	storage := persistentStorage{
		// TODO: consider using NewInMemoryStorage(..), then no need for creating PollData when creating a file.
		inMemoryStorage: inMemoryStorage{
			logger: logger,
		},
		filename: filename,
	}

	if _, err := os.Stat(filename); errors.Is(err, os.ErrNotExist) {
		err = storage.Save(PollData{
			CheckPoint:     time.Now(),
			NextCheckPoint: nil,
			ToDate:         nil,
		})
		if err != nil {
			return nil, fmt.Errorf("saving poll data configuration file: %w", err)
		}
		logger.Info("new persistent storage was created", zap.Any("filename", storage.filename), zap.Any("poll_data", storage.inMemoryStorage.pollData))

		return &storage, nil
	}

	jsonFile, err := os.Open(filename)
	if err != nil {
		return nil, fmt.Errorf("opening poll data configuration file: %w", err)
	}
	defer jsonFile.Close()

	byteValue, err := io.ReadAll(jsonFile)
	if err != nil {
		return nil, fmt.Errorf("reading poll data configuration file: %w", err)
	}

	err = json.Unmarshal(byteValue, &storage.inMemoryStorage.pollData)
	if err != nil {
		return nil, fmt.Errorf("parsing poll data configuration file: %w", err)
	}

	// Format validation is done by JSON unmarshaller, so here it is only 'semantic' validations.
	err = storage.validate()
	if err != nil {
		return nil, fmt.Errorf("validating poll data configuration file: %w", err)
	}

	logger.Info("loaded persistent storage configuration", zap.Any("filename", storage.filename), zap.Any("poll_data", storage.inMemoryStorage.pollData))

	return &storage, nil
}

func (s *persistentStorage) Get() PollData {
	return s.pollData
}

func (s *persistentStorage) Save(data PollData) error {
	s.pollData = data

	jsonBytes, err := json.Marshal(&s.inMemoryStorage.pollData)
	if err != nil {
		return err
	}

	err = os.WriteFile(s.filename, jsonBytes, os.ModePerm)
	if err != nil {
		return err
	}

	return nil
}

func (s *persistentStorage) validate() error {
	if s.inMemoryStorage.pollData.NextCheckPoint != nil {
		if s.inMemoryStorage.pollData.ToDate == nil {
			return fmt.Errorf("to_date must be provided when next_check_point date is present")
		}

		if s.inMemoryStorage.pollData.NextCheckPoint.Before(s.inMemoryStorage.pollData.CheckPoint) {
			return fmt.Errorf("next_check_point date must succeed check_point")
		}
		if s.inMemoryStorage.pollData.ToDate.Before(s.inMemoryStorage.pollData.CheckPoint) {
			return fmt.Errorf("to_date date must succeed check_point")
		}

		if s.inMemoryStorage.pollData.NextCheckPoint.Before(*s.inMemoryStorage.pollData.ToDate) {
			return fmt.Errorf("next_check_point date must succeed or be equal to to_date")
		}
	}

	return nil
}
