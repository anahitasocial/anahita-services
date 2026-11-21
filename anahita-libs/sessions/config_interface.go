package sessions

type ConfigInterface interface {
	GetSecureCookieKey() string
	GetValkeyAddr() string
	GetValkeyPassword() string
	GetValkeyDB() int
	IsStaging() bool
	IsDev() bool
	IsProd() bool
	GetDomainName() string
}
