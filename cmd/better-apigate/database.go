package main

import (
	"fmt"
	"os"

	"github.com/coolqoo/better-apigate/adapters/postgres"
	"github.com/coolqoo/better-apigate/config"
)

// databaseDSN overrides deployment configuration for an administrative command.
var databaseDSN string

func openDatabase() (*postgres.DB, error) {
	dsn := databaseDSN
	if dsn == "" {
		dsn = os.Getenv("APIGATE_DATABASE_DSN")
	}
	if dsn == "" {
		cfg, err := config.Load(cfgFile)
		if err != nil {
			return nil, fmt.Errorf("load configuration: %w; set APIGATE_DATABASE_DSN to the PostgreSQL connection URL", err)
		}
		dsn = cfg.Database.DSN
	}
	db, err := postgres.Open(dsn)
	if err != nil {
		return nil, err
	}
	if err = db.Migrate(); err != nil {
		db.Close()
		return nil, err
	}
	return db, nil
}
