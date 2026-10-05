package email

import (
	"fmt"
	"time"

	"github.com/coolqoo/better-apigate/domain/settings"
	"github.com/coolqoo/better-apigate/ports"
)

// NewSender creates an email sender based on settings.
func NewSender(s settings.Settings) (ports.EmailSender, error) {
	provider := s.Get(settings.KeyEmailProvider)

	switch provider {
	case "smtp":
		port := s.GetInt(settings.KeyEmailSMTPPort, 587)
		useTLS := true
		if s.Get(settings.KeyEmailSMTPUseTLS) != "" {
			useTLS = s.GetBool(settings.KeyEmailSMTPUseTLS)
		}
		config := SMTPConfig{
			Host:        s.Get(settings.KeyEmailSMTPHost),
			Port:        port,
			Username:    s.Get(settings.KeyEmailSMTPUsername),
			Password:    s.Get(settings.KeyEmailSMTPPassword),
			From:        s.Get(settings.KeyEmailFromAddress),
			FromName:    s.Get(settings.KeyEmailFromName),
			UseTLS:      useTLS,
			UseImplicit: port == 465,
			Timeout:     30 * time.Second,
			BaseURL:     s.Get(settings.KeyPortalBaseURL),
			AppName:     s.GetOrDefault(settings.KeyPortalAppName, "better-apigate"),
		}
		if config.Host == "" {
			return nil, fmt.Errorf("SMTP host is required")
		}
		return NewSMTPSender(config)

	case "mock":
		baseURL := s.Get(settings.KeyPortalBaseURL)
		appName := s.GetOrDefault(settings.KeyPortalAppName, "better-apigate")
		return NewMockSender(baseURL, appName), nil

	case "none", "":
		return NewNoopSender(), nil

	default:
		return nil, fmt.Errorf("unknown email provider: %s", provider)
	}
}
