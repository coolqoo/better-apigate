package main

import (
	"context"
	"os"

	"github.com/coolqoo/better-apigate/adapters/postgres"
	"github.com/coolqoo/better-apigate/bootstrap"
	"github.com/rs/zerolog"
	"github.com/spf13/cobra"
)

var modCmd *cobra.Command
var moduleRuntime *bootstrap.ModuleRuntime
var moduleDB *postgres.DB

func init() {
	modCmd = &cobra.Command{
		Use:   "mod",
		Short: "Module-based management commands",
		Long: `Access declarative module CRUD operations.

These commands use the module system to manage entities:
  better-apigate mod users list
  better-apigate mod plans get free
  better-apigate mod upstreams create --name "API" --base_url "https://api.example.com"

Available subcommands are generated from loaded modules.`,
		PersistentPostRun: func(cmd *cobra.Command, args []string) {
			// Flush and close analytics when command completes
			if moduleRuntime != nil {
				moduleRuntime.Stop(context.Background())
				if moduleDB != nil {
					moduleDB.Close()
				}
			}
		},
	}
	rootCmd.AddCommand(modCmd)

	// Try to initialize modules at startup for help text
	// This is best-effort - errors are silently ignored
	if len(os.Args) > 1 && os.Args[1] == "mod" {
		tryInitModules()
	}
}

func tryInitModules() {
	// Setup quiet logger
	logger := zerolog.New(os.Stdout).Level(zerolog.ErrorLevel).With().Timestamp().Logger()

	// Get database path
	dsn := os.Getenv("APIGATE_DATABASE_DSN")
	if dsn == "" {
		return
	}

	// Open database
	db, err := postgres.Open(dsn)
	if err != nil {
		return
	}

	if err := db.Migrate(); err != nil {
		db.Close()
		return
	}

	// Create module runtime with modCmd as root
	mr, err := bootstrap.NewModuleRuntime(db.DB, modCmd, logger, bootstrap.ModuleConfig{})
	if err != nil {
		db.Close()
		return
	}

	// Load modules
	ctx := context.Background()
	if err := mr.LoadModules(ctx, bootstrap.ModuleConfig{
		EmbeddedModules: bootstrap.CoreModules(),
	}); err != nil {
		db.Close()
		return
	}

	moduleDB = db
	moduleRuntime = mr
}
