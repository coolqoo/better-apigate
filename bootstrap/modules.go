// Package bootstrap - modules.go provides integration for the declarative module system.
package bootstrap

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"path/filepath"

	"github.com/coolqoo/better-apigate/core/analytics"
	cliChannel "github.com/coolqoo/better-apigate/core/channel/cli"
	httpChannel "github.com/coolqoo/better-apigate/core/channel/http"
	"github.com/coolqoo/better-apigate/core/convention"
	"github.com/coolqoo/better-apigate/core/exporter"
	yamlmodules "github.com/coolqoo/better-apigate/core/modules"
	"github.com/coolqoo/better-apigate/core/registry"
	"github.com/coolqoo/better-apigate/core/runtime"
	"github.com/coolqoo/better-apigate/core/schema"
	"github.com/coolqoo/better-apigate/core/storage"
	"github.com/rs/zerolog"
	"github.com/spf13/cobra"
)

// ModuleRuntime wraps the declarative module runtime with app integration.
type ModuleRuntime struct {
	Runtime   *runtime.Runtime
	Registry  *registry.Registry
	Storage   *storage.PostgresStore
	Analytics *analytics.PostgresStore
	HTTP      *httpChannel.Channel
	CLI       *cliChannel.Channel
	Logger    zerolog.Logger

	modules []schema.Module
}

// ModuleConfig configures module loading.
type ModuleConfig struct {
	// ModulesDir is the directory containing module YAML files.
	// Defaults to "modules" relative to working directory.
	ModulesDir string

	// PluginsDir is the directory containing plugin modules.
	PluginsDir string

	// EmbeddedModules are modules defined in code (for core modules).
	EmbeddedModules []schema.Module
}

// NewModuleRuntime creates a new module runtime using an existing database.
func NewModuleRuntime(db *sql.DB, rootCmd *cobra.Command, logger zerolog.Logger, cfg ModuleConfig) (*ModuleRuntime, error) {
	mr := &ModuleRuntime{
		Logger: logger,
	}

	// Create storage adapter from existing DB
	mr.Storage = storage.NewPostgresStoreFromDB(db)

	// Create analytics store
	analyticsStore, err := analytics.NewPostgresStore(db, analytics.DefaultPostgresConfig())
	if err != nil {
		return nil, fmt.Errorf("create analytics store: %w", err)
	}
	mr.Analytics = analyticsStore

	// Create storage adapter for runtime
	adapter := &runtimeStorageAdapter{store: mr.Storage}

	// Create runtime with analytics and logger
	mr.Runtime = runtime.New(adapter, runtime.Config{
		ModulesDir: cfg.ModulesDir,
		PluginsDir: cfg.PluginsDir,
		Analytics:  analyticsStore,
		Logger:     logger,
	})
	mr.Registry = mr.Runtime.Registry()

	// Register built-in functions and module hooks
	RegisterHooks(mr.Runtime, logger)

	// Register default Prometheus exporter
	promExporter := exporter.NewPrometheusExporter(exporter.PrometheusConfig{
		Store:  analyticsStore,
		Prefix: "apigate",
	})
	if err := mr.Runtime.RegisterExporter(promExporter); err != nil {
		logger.Warn().Err(err).Msg("failed to register prometheus exporter")
	} else {
		logger.Debug().Msg("prometheus exporter registered")
	}

	// Create HTTP channel (will be mounted later)
	mr.HTTP = httpChannel.New(mr.Runtime, "")

	// Create CLI channel
	mr.CLI = cliChannel.New(rootCmd, mr.Runtime)

	// Register channels with runtime
	mr.Runtime.RegisterChannel(mr.CLI)
	mr.Runtime.RegisterChannel(mr.HTTP)

	return mr, nil
}

// LoadModules loads modules from the configured directories and embedded modules.
func (mr *ModuleRuntime) LoadModules(ctx context.Context, cfg ModuleConfig) error {
	// Load embedded modules first
	for _, mod := range cfg.EmbeddedModules {
		if err := mr.loadModule(ctx, mod); err != nil {
			return fmt.Errorf("load embedded module %s: %w", mod.Name, err)
		}
	}

	// Load modules from directory
	if cfg.ModulesDir != "" {
		if err := mr.loadModulesFromDir(ctx, cfg.ModulesDir); err != nil {
			mr.Logger.Warn().Err(err).Str("dir", cfg.ModulesDir).Msg("failed to load modules from directory")
		}
	}

	// Load plugin modules
	if cfg.PluginsDir != "" {
		if err := mr.loadModulesFromDir(ctx, cfg.PluginsDir); err != nil {
			mr.Logger.Warn().Err(err).Str("dir", cfg.PluginsDir).Msg("failed to load plugin modules")
		}
	}

	mr.Logger.Info().Int("count", len(mr.modules)).Msg("modules loaded")
	return nil
}

func (mr *ModuleRuntime) loadModulesFromDir(ctx context.Context, dir string) error {
	modules, err := schema.ParseDir(dir)
	if err != nil {
		return fmt.Errorf("parse modules from %q: %w", dir, err)
	}

	for _, mod := range modules {
		if err := mr.loadModule(ctx, mod); err != nil {
			// Expected for core modules that exist as both Go and YAML definitions
			mr.Logger.Debug().Err(err).Str("module", mod.Name).Msg("failed to load module")
			continue
		}
	}

	return nil
}

func (mr *ModuleRuntime) loadModule(ctx context.Context, mod schema.Module) error {
	// Validate module
	if err := schema.Validate(mod); err != nil {
		return fmt.Errorf("validate module %q: %w", mod.Name, err)
	}

	// Load into runtime (may fail if already registered)
	if err := mr.Runtime.LoadModule(mod); err != nil {
		// If module already registered, still register its hooks
		// This handles the case where embedded modules load first,
		// then YAML versions with hooks are loaded after
		if len(mod.Hooks) > 0 {
			mr.Runtime.RegisterModuleHooks(mod)
			mr.Logger.Info().
				Str("module", mod.Name).
				Int("hooks", countHooks(mod.Hooks)).
				Msg("registered YAML hooks for module")
		}
		return fmt.Errorf("load module %q: %w", mod.Name, err)
	}

	// Register YAML-declared hooks for this module
	mr.Runtime.RegisterModuleHooks(mod)

	mr.modules = append(mr.modules, mod)
	mr.Logger.Debug().Str("module", mod.Name).Msg("loaded module")
	return nil
}

// countHooks counts total hooks across all phases
func countHooks(hooks map[string][]schema.Hook) int {
	count := 0
	for _, h := range hooks {
		count += len(h)
	}
	return count
}

// Handler returns an HTTP handler for all module endpoints.
// This should be mounted at a base path like /api/v2 or /modules.
func (mr *ModuleRuntime) Handler() http.Handler {
	return mr.HTTP.Handler()
}

// MetricsHandler returns an HTTP handler for the /metrics endpoint.
// Returns nil if no pull exporters are registered.
func (mr *ModuleRuntime) MetricsHandler() http.Handler {
	exporters := mr.Runtime.Exporters().PullExporters()
	if len(exporters) == 0 {
		return nil
	}
	// Return the first pull exporter's handler (typically Prometheus)
	return exporters[0].Handler()
}

// AuthHandler returns the auth routes handler for mounting at additional paths.
// This enables SPA frontends to access auth at /api/portal/auth/*.
func (mr *ModuleRuntime) AuthHandler() http.Handler {
	return mr.HTTP.AuthRoutes()
}

// Start starts the module runtime (channels, hooks, etc.).
func (mr *ModuleRuntime) Start(ctx context.Context) error {
	return mr.Runtime.Start(ctx)
}

// Stop stops the module runtime.
func (mr *ModuleRuntime) Stop(ctx context.Context) error {
	// Stop runtime first
	if err := mr.Runtime.Stop(ctx); err != nil {
		return err
	}
	// Close analytics (flushes pending events)
	if mr.Analytics != nil {
		return mr.Analytics.Close()
	}
	return nil
}

// Modules returns the list of loaded modules.
func (mr *ModuleRuntime) Modules() []schema.Module {
	return mr.modules
}

// GetModule returns a specific module by name.
func (mr *ModuleRuntime) GetModule(name string) (convention.Derived, bool) {
	return mr.Registry.Get(name)
}

// Execute executes an action on a module.
func (mr *ModuleRuntime) Execute(ctx context.Context, module, action string, input runtime.ActionInput) (runtime.ActionResult, error) {
	return mr.Runtime.Execute(ctx, module, action, input)
}

// GetHTTPPaths returns all HTTP paths registered by modules.
func (mr *ModuleRuntime) GetHTTPPaths() []schema.PathClaim {
	return mr.Registry.GetHTTPPaths()
}

// GetCLIPaths returns all CLI paths registered by modules.
func (mr *ModuleRuntime) GetCLIPaths() []schema.PathClaim {
	return mr.Registry.GetCLIPaths()
}

// runtimeStorageAdapter adapts storage.PostgresStore to runtime.Storage.
type runtimeStorageAdapter struct {
	store *storage.PostgresStore
}

func (a *runtimeStorageAdapter) CreateTable(ctx context.Context, mod convention.Derived) error {
	return a.store.CreateTable(ctx, mod)
}

func (a *runtimeStorageAdapter) Create(ctx context.Context, module string, data map[string]any) (string, error) {
	return a.store.Create(ctx, module, data)
}

func (a *runtimeStorageAdapter) Get(ctx context.Context, module string, lookup string, value string) (map[string]any, error) {
	return a.store.Get(ctx, module, lookup, value)
}

func (a *runtimeStorageAdapter) List(ctx context.Context, module string, opts runtime.ListOptions) ([]map[string]any, int64, error) {
	return a.store.List(ctx, module, storage.ListOptions{
		Limit:     opts.Limit,
		Offset:    opts.Offset,
		Filters:   opts.Filters,
		OrderBy:   opts.OrderBy,
		OrderDesc: opts.OrderDesc,
	})
}

func (a *runtimeStorageAdapter) Update(ctx context.Context, module string, id string, data map[string]any) error {
	return a.store.Update(ctx, module, id, data)
}

func (a *runtimeStorageAdapter) Delete(ctx context.Context, module string, id string) error {
	return a.store.Delete(ctx, module, id)
}

// CoreModules returns the core module definitions that are embedded in the application.
// These define the standard user, plan, api_key, route, upstream, and setting modules.
// Note: Analytics is a runtime capability, not a data module.
func CoreModules() []schema.Module {
	mods, err := yamlmodules.Core()
	if err != nil {
		panic(fmt.Sprintf("invalid embedded module definition: %v", err))
	}
	return mods
}

// CoreModulesDir returns the path to the core modules directory.
func CoreModulesDir() string {
	// This returns a path relative to the binary or working directory
	// In production, modules would be embedded or in a known location
	return filepath.Join("core", "modules")
}
