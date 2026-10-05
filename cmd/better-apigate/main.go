// Package main is the entry point for better-apigate.
//
//	@title						better-apigate - API Monetization Proxy
//	@version					1.0
//	@description				Self-hosted API monetization solution with authentication, rate limiting, usage metering, and billing.
//	@termsOfService				https://github.com/coolqoo/better-apigate
//
//	@contact.name				better-apigate Support
//	@contact.url				https://github.com/coolqoo/better-apigate/issues
//
//	@license.name				MIT
//	@license.url				https://opensource.org/licenses/MIT
//
//	@host						localhost:8080
//	@BasePath					/
//
//	@securityDefinitions.apikey	ApiKeyAuth
//	@in							header
//	@name						X-API-Key
//	@description				API key for authentication
//
//	@securityDefinitions.apikey	BearerAuth
//	@in							header
//	@name						Authorization
//	@description				Bearer token authentication (format: "Bearer {api_key}")
package main

func main() {
	Execute()
}
