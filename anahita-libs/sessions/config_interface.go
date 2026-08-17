package sessions

type ConfigInterface interface {
	GetSecureCookieKey() string
	GetRedisAddr() string
	GetRedisPassword() string
	GetRedisDB() int
	IsStaging() bool
	IsDev() bool
	IsProd() bool
	GetDomainName() string
}
