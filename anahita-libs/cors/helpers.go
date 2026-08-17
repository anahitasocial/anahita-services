package cors

import "fmt"

type ConfigInterface interface {
	IsDev() bool
	IsStaging() bool
	IsProd() bool
	GetDomainName() string
}

func GetAllowedOrigins(config ConfigInterface) []string {
	var allowedOrigins []string

	domainName := config.GetDomainName()

	if config.IsProd() {
		allowedOrigins = append(allowedOrigins, fmt.Sprintf("https://www.%s", domainName))
		allowedOrigins = append(allowedOrigins, fmt.Sprintf("https://main.%s", domainName))
	} else if config.IsStaging() {
		allowedOrigins = append(allowedOrigins, fmt.Sprintf("https://staging.%s", domainName))
		allowedOrigins = append(allowedOrigins, fmt.Sprintf("https://main.staging.%s", domainName))
	} else if config.IsDev() {
		allowedOrigins = append(allowedOrigins, "http://localhost:1212")
		allowedOrigins = append(allowedOrigins, "http://localhost:3000")
	}

	return allowedOrigins
}
