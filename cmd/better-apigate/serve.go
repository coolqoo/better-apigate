package main

import (
	"fmt"

	"github.com/coolqoo/better-apigate/bootstrap"
	"github.com/spf13/cobra"
)

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start the API proxy server",
	Long: `Start the better-apigate proxy server.

All configuration is loaded from the database after connection.
Only minimal bootstrap settings come from environment variables:

Environment variables:
  APIGATE_DATABASE_DSN  - Required PostgreSQL connection URL
  APIGATE_REDIS_URL     - Required Redis connection URL
  APIGATE_API_KEY_SECRET - Required deployment-managed key secret (32+ characters)
  APIGATE_PUBLIC_URL    - Browser origin for cookies, CSRF and checkout returns
  APIGATE_SETUP_TOKEN   - Deployment setup token for the initial administrator
  APIGATE_METRICS_TOKEN - Optional protected Prometheus scrape credential
  APIGATE_SERVER_HOST   - Server host (default: from settings or 0.0.0.0)
  APIGATE_SERVER_PORT   - Server port (default: from settings or 8080)
  APIGATE_LOG_LEVEL     - Log level: debug, info, warn, error
  APIGATE_LOG_FORMAT    - Log format: json or console

All other settings are stored in the database and can be configured
via the admin UI or API:
  - Email provider settings (SMTP, etc.)
  - Payment providers (Stripe, Paddle, Lemon Squeezy, EPUSDT)
  - Portal settings
  - Rate limit settings
  - Upstream settings

Examples:
  better-apigate serve
  APIGATE_DATABASE_DSN=postgres://apigate@localhost:5432/apigate better-apigate serve
  APIGATE_LOG_LEVEL=debug APIGATE_LOG_FORMAT=console better-apigate serve`,
	RunE: runServe,
}

func init() {
	rootCmd.AddCommand(serveCmd)
}

func runServe(cmd *cobra.Command, args []string) error {
	// Log version info at startup to help diagnose issues with stale builds (#32)
	fmt.Printf("better-apigate %s (commit: %s, built: %s)\n", version, commit, buildDate)

	// Create application with root command for module CLI integration
	app, err := bootstrap.NewWithConfig(bootstrap.Config{
		RootCmd: rootCmd,
	})
	if err != nil {
		return fmt.Errorf("error initializing: %w", err)
	}

	// Run (blocks until shutdown)
	return app.Run()
}
