package services

import "file/config"

func NewServices(config *config.Config) *Services {
	return &Services{
		AwsS3: NewAwsS3Service(config),
	}
}

type Services struct {
	AwsS3 AwsS3ServiceInterface
}
