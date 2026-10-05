package main

import (
	"bufio"
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"github.com/google/uuid"
	"net/mail"
	"net/url"
	"os"
	"strconv"
	"strings"

	"github.com/coolqoo/better-apigate/adapters/hasher"
	"github.com/coolqoo/better-apigate/adapters/postgres"
	"github.com/coolqoo/better-apigate/domain/key"
	"github.com/coolqoo/better-apigate/domain/settings"
	"github.com/spf13/cobra"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Interactive setup wizard",
	Long: `Initialize better-apigate with an interactive setup wizard.

This will:
  1. Ask for your upstream API URL
  2. Configure a PostgreSQL connection URL
  3. Create initial configuration file
  4. Create admin user (optional)
  5. Generate an optional API key using the deployment key secret

Examples:
  better-apigate init
  better-apigate init --config /etc/better-apigate/config.yaml`,
	RunE: runInit,
}

var (
	initUpstream       string
	initDatabase       string
	initAdminEmail     string
	initAdminPassword  string
	initNonInteractive bool
)

func init() {
	rootCmd.AddCommand(initCmd)

	initCmd.Flags().StringVar(&initUpstream, "upstream", "", "upstream API URL")
	initCmd.Flags().StringVar(&initDatabase, "database", "", "PostgreSQL connection URL")
	initCmd.Flags().StringVar(&initAdminEmail, "admin-email", "", "admin user email")
	initCmd.Flags().StringVar(&initAdminPassword, "admin-password", "", "admin user password (auto-generated if not provided)")
	initCmd.Flags().BoolVar(&initNonInteractive, "non-interactive", false, "run without prompts (requires --upstream)")
}

func runInit(cmd *cobra.Command, args []string) error {
	fmt.Println("Welcome to better-apigate!")
	fmt.Println()

	// Check if config already exists
	if _, err := os.Stat(cfgFile); err == nil {
		fmt.Printf("Configuration file already exists: %s\n", cfgFile)
		if !confirm("Overwrite?") {
			fmt.Println("Aborted.")
			return nil
		}
	}

	reader := bufio.NewReader(os.Stdin)

	// Get upstream URL
	upstream := initUpstream
	if upstream == "" {
		if initNonInteractive {
			return fmt.Errorf("--upstream is required in non-interactive mode")
		}
		upstream = prompt(reader, "Upstream API URL", "")
		if upstream == "" {
			return fmt.Errorf("upstream URL is required")
		}
	}

	// Get database location
	database := initDatabase
	if database == "" {
		database = os.Getenv("APIGATE_DATABASE_DSN")
	}
	if !initNonInteractive && database == "" {
		database = prompt(reader, "PostgreSQL URL", "postgres://apigate@localhost:5432/apigate?sslmode=disable")
	}

	// Create admin user?
	var adminEmail string
	var adminPassword string
	createAdmin := false
	if initAdminEmail != "" {
		adminEmail = initAdminEmail
		adminPassword = initAdminPassword
		createAdmin = true
	} else if !initNonInteractive {
		createAdmin = confirm("Create admin user?")
		if createAdmin {
			adminEmail = prompt(reader, "Admin email", "")
			if adminEmail == "" {
				return fmt.Errorf("admin email is required")
			}
			// Prompt for password
			adminPassword, _ = promptPassword("Admin password (leave empty to auto-generate)")
		}
	}

	// Generate password if not provided
	if createAdmin && adminPassword == "" {
		adminPassword = generatePassword()
	}

	u, err := url.Parse(upstream)
	if err != nil || u.Host == "" || u.User != nil || (u.Scheme != "http" && u.Scheme != "https") {
		return fmt.Errorf("use an absolute HTTP upstream URL")
	}
	if createAdmin {
		address, err := mail.ParseAddress(adminEmail)
		if err != nil || address.Address != adminEmail || len(adminPassword) < 12 || len(adminPassword) > 72 {
			return fmt.Errorf("use a valid administrator email and password of 12–72 characters")
		}
		if len(os.Getenv("APIGATE_API_KEY_SECRET")) < 32 {
			return fmt.Errorf("APIGATE_API_KEY_SECRET requires at least 32 characters")
		}
	}
	// Generate config
	configContent := generateConfig(upstream, database)

	// Write config file
	if err := os.WriteFile(cfgFile, []byte(configContent), 0600); err != nil {
		return fmt.Errorf("failed to write config: %w", err)
	}
	fmt.Printf("\n%s Generated %s\n", checkMark, cfgFile)

	// Create database and run migrations
	db, err := postgres.Open(database)
	if err != nil {
		return fmt.Errorf("failed to create database: %w", err)
	}
	defer db.Close()

	if err := db.Migrate(); err != nil {
		return fmt.Errorf("failed to migrate database: %w", err)
	}
	fmt.Printf("%s Initialized PostgreSQL database\n", checkMark)

	// Save upstream URL and other settings to database
	settingsStore := postgres.NewSettingsStore(db)
	ctx := context.Background()

	// Save upstream URL
	if err := settingsStore.Set(ctx, settings.KeyUpstreamURL, upstream, false); err != nil {
		return fmt.Errorf("failed to save upstream URL: %w", err)
	}

	// Enable portal by default
	if err := settingsStore.Set(ctx, settings.KeyPortalEnabled, "true", false); err != nil {
		return fmt.Errorf("failed to enable portal: %w", err)
	}

	fmt.Printf("%s Saved settings to database\n", checkMark)

	// Create admin user if requested
	if createAdmin && adminEmail != "" {
		apiKey, err := createAdminUser(db, adminEmail, adminPassword)
		if err != nil {
			return fmt.Errorf("failed to create admin user: %w", err)
		}
		fmt.Printf("%s Created admin user: %s\n", checkMark, adminEmail)
		fmt.Println()
		fmt.Println("Admin credentials (save these, shown once):")
		fmt.Printf("  Email:    %s\n", adminEmail)
		fmt.Printf("  Password: %s\n", adminPassword)
		fmt.Printf("  API Key:  %s\n", apiKey)
	}

	fmt.Println()
	fmt.Println("Set APIGATE_DATABASE_DSN, APIGATE_REDIS_URL, APIGATE_API_KEY_SECRET and APIGATE_PUBLIC_URL, then run 'better-apigate serve'.")
	fmt.Println()
	fmt.Println("Access points:")
	fmt.Println("  Admin Dashboard: http://localhost:8080/login")
	fmt.Println("  User Portal:     http://localhost:8080/portal")
	fmt.Println("  API Proxy:       http://localhost:8080/ (requires API key)")

	return nil
}

func prompt(reader *bufio.Reader, label, defaultVal string) string {
	if defaultVal != "" {
		fmt.Printf("? %s [%s]: ", label, defaultVal)
	} else {
		fmt.Printf("? %s: ", label)
	}

	input, _ := reader.ReadString('\n')
	input = strings.TrimSpace(input)

	if input == "" {
		return defaultVal
	}
	return input
}

func confirm(message string) bool {
	reader := bufio.NewReader(os.Stdin)
	fmt.Printf("? %s [y/N]: ", message)
	input, _ := reader.ReadString('\n')
	input = strings.ToLower(strings.TrimSpace(input))
	return input == "y" || input == "yes"
}

func generateConfig(upstream, database string) string {
	return fmt.Sprintf(`# better-apigate Configuration
# Generated by 'better-apigate init'

server:
  host: "0.0.0.0"
  port: 8080

upstream:
  url: %s
  timeout: 30s

database:
  driver: postgres
  dsn: %s

auth:
  mode: local
  key_prefix: "ak_"

rate_limit:
  enabled: true
  burst_tokens: 10
  window_secs: 60

logging:
  level: info
  format: console

metrics:
  enabled: true

openapi:
  enabled: true
`, strconv.Quote(upstream), strconv.Quote(database))
}

func createAdminUser(db *postgres.DB, email, password string) (string, error) {
	address, err := mail.ParseAddress(email)
	secret := []byte(os.Getenv("APIGATE_API_KEY_SECRET"))
	if err != nil || address.Address != email || len(password) < 12 || len(password) > 72 || len(secret) < 32 {
		return "", fmt.Errorf("valid email, password and deployment key secret are required")
	}
	ctx := context.Background()

	h := hasher.NewBcrypt(10)
	passwordHash, err := h.Hash(password)
	if err != nil {
		return "", fmt.Errorf("hash password: %w", err)
	}
	rawKey, keyData := key.Generate("ak_", secret)
	userID := generateID()
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return "", err
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, "INSERT INTO users(id,email,name,password_hash,role,plan_id,status,email_verified) VALUES(?,?,?,?,'admin','paygo','active',TRUE)", userID, strings.ToLower(email), "Administrator", passwordHash); err != nil {
		return "", fmt.Errorf("create user: %w", err)
	}
	if _, err = tx.ExecContext(ctx, "INSERT INTO api_keys(id,user_id,hash,prefix,name) VALUES(?,?,?,?,?)", keyData.ID, userID, keyData.Hash, keyData.Prefix, "Initial API key"); err != nil {
		return "", fmt.Errorf("create key: %w", err)
	}
	if err = tx.Commit(); err != nil {
		return "", err
	}

	return rawKey, nil
}

func generatePassword() string {
	bytes := make([]byte, 12)
	rand.Read(bytes)
	return hex.EncodeToString(bytes)[:16]
}

func generateID() string {
	// Use a unique identifier across independently initialized deployments.
	return uuid.NewString()
}
