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
	Save(PollData) error
	Get() PollData
}

type inMemoryStorage struct {
	logger   *zap.Logger
	pollData PollData
}

func NewInMemoryStorage(logger *zap.Logger, data PollData) Storage {
	return &inMemoryStorage{
		logger:   logger,
		pollData: data,
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

func NewPersistentStorage(logger *zap.Logger, filename string) Storage {
	storage := persistentStorage{
		inMemoryStorage: inMemoryStorage{
			logger: logger,
		},
		filename: filename,
	}
	err := storage.load()
	if err != nil {
		fmt.Println(err)
	}

	return &storage
}

func (s *persistentStorage) Get() PollData {
	return s.pollData
}

func (s *persistentStorage) Save(data PollData) error {
	s.pollData = data
	return s.save()
}

func (s *persistentStorage) save() error {
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

func (s *persistentStorage) load() error {
	if _, err := os.Stat(s.filename); errors.Is(err, os.ErrNotExist) {
		// TODO: logging
		return s.inMemoryStorage.Save(PollData{
			CheckPoint:     time.Now().UTC(),
			NextCheckPoint: nil,
			ToDate:         nil,
		})
	}

	jsonFile, err := os.Open(s.filename)
	if err != nil {
		fmt.Println(err)
	}
	defer jsonFile.Close()

	byteValue, err := io.ReadAll(jsonFile)
	if err != nil {
		fmt.Println(err)
	}

	err = json.Unmarshal(byteValue, &s.inMemoryStorage.pollData)
	if err != nil {
		fmt.Println(err)
	}

	// TODO: file content validation

	return nil
}
