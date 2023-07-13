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

type Data struct {
	CheckPoint     time.Time
	NextCheckPoint *time.Time
	ToDate         *time.Time
}

type Storage interface {
	Save(Data) error
	Get() Data
}

type inMemoryStorage struct {
	logger *zap.Logger
	data   Data
}

func NewInMemoryStorage(logger *zap.Logger, data Data) Storage {
	return &inMemoryStorage{
		logger: logger,
		data:   data,
	}
}

func (s *inMemoryStorage) Get() Data {
	return s.data
}

func (s *inMemoryStorage) Save(data Data) error {
	s.data = data
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

func (s *persistentStorage) Get() Data {
	return s.data
}

func (s *persistentStorage) Save(data Data) error {
	s.data = data
	return s.save()
}

func (s *persistentStorage) save() error {
	jsonBytes, err := json.Marshal(&s.inMemoryStorage.data)
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
		return s.inMemoryStorage.Save(Data{
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

	err = json.Unmarshal(byteValue, &s.inMemoryStorage.data)
	if err != nil {
		fmt.Println(err)
	}

	// TODO: file content validation

	return nil
}
