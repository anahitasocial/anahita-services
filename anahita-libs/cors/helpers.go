package cors

type ConfigInterface interface {
	IsDev() bool
	IsStaging() bool
	IsProd() bool
	GetDomainName() string
}
