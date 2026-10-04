package services

import "file/config"

func NewServices(config *config.Config) (*Services, error) {
	storage, err := NewStorage(config)
	if err != nil {
		return nil, err
	}

	return &Services{Storage: storage}, nil
}

type Services struct {
	Storage StorageInterface
}
