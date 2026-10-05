package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var (
	// Global flags
	cfgFile string
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "better-apigate",
	Short: "API monetization proxy with authentication, rate limiting, and billing",
	Long: `better-apigate is a self-hosted API monetization solution.

It provides authentication, rate limiting, usage metering, and billing
for your APIs. Deploy in front of any API to add monetization.

Quick start:
  better-apigate init      # Interactive setup wizard
  better-apigate serve     # Start the proxy server

Management:
  better-apigate mod      # Manage gateway configuration from YAML modules
  better-apigate admin    # Manage administrators
  better-apigate validate  # Validate configuration`,
}

// Execute adds all child commands to the root command and sets flags appropriately.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	// Global flags
	rootCmd.PersistentFlags().StringVarP(&cfgFile, "config", "c", "better-apigate.yaml", "config file path")
}
