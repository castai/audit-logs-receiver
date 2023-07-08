package storage

type Store interface {
	Kuku() error
}

type SampleStore struct{}

func NewSampleStore() Store {
	return SampleStore{}
}

func (s SampleStore) Kuku() error {
	//TODO implement me
	panic("implement me")
}
