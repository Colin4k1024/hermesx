// Package main implements the hermesx CLI entry point using Cobra.
package main

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/Colin4k1024/hermesx/internal/cli"
	"github.com/Colin4k1024/hermesx/internal/config"
	"github.com/spf13/cobra"
)

// Build-time variables set via ldflags.
var (
	version     = "dev"
	releaseDate = "unknown"
	commit      = "none"
)

func init() {
	cli.Version = version
	cli.ReleaseDate = releaseDate
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "fatal: %v\n", err)
		os.Exit(1)
	}
}

// --- Persistent flags ---
var (
	flagDebug bool
)

// --- Root command ---

var rootCmd = &cobra.Command{
	Use:   "hermesx",
	Short: "HermesX - SaaS control plane",
	Long: `HermesX is a multi-tenant SaaS control plane for governed agent automation.
The supported runtime entry point is the SaaS API server: hermesx saas-api.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return fmt.Errorf("%s: local standalone mode has been removed; use `hermesx saas-api` and the SaaS HTTP API instead", cmd.CommandPath())
	},
	SilenceUsage:  true,
	SilenceErrors: true,
	CompletionOptions: cobra.CompletionOptions{
		DisableDefaultCmd: true,
	},
}

func init() {
	rootCmd.PersistentFlags().BoolVar(&flagDebug, "debug", false, "Enable debug logging")
	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(clawCmd)
}

func setupLogging() {
	if flagProfile != "" {
		config.OverrideActiveProfile(flagProfile)
	}

	level := slog.LevelInfo
	if flagDebug || os.Getenv("HERMES_DEBUG") != "" {
		level = slog.LevelDebug
	}
	opts := &slog.HandlerOptions{Level: level}

	var handler slog.Handler
	logFmt := os.Getenv("LOG_FORMAT")
	env := os.Getenv("HERMES_ENV")
	if logFmt == "json" || env == "production" {
		handler = slog.NewJSONHandler(os.Stderr, opts)
	} else {
		handler = slog.NewTextHandler(os.Stderr, opts)
	}
	slog.SetDefault(slog.New(handler))
}

// flagProfile is used by setupLogging (called from saas.go).
var flagProfile string

// --- Version command ---

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Show version information",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("HermesX v%s (%s)\n", version, releaseDate)
		fmt.Printf("Commit: %s\n", commit)
	},
}

// --- Claw (OpenClaw migration) command ---

var clawCmd = &cobra.Command{
	Use:   "claw",
	Short: "OpenClaw migration tools",
}

var clawMigrateCmd = &cobra.Command{
	Use:   "migrate",
	Short: "Migrate settings and data from OpenClaw to Hermes",
	RunE: func(cmd *cobra.Command, args []string) error {
		_ = config.EnsureHermesHome()
		dryRun, _ := cmd.Flags().GetBool("dry-run")
		overwrite, _ := cmd.Flags().GetBool("overwrite")
		return cli.RunClawMigrate(dryRun, overwrite)
	},
}

func init() {
	clawMigrateCmd.Flags().Bool("dry-run", false, "Show what would be migrated without making changes")
	clawMigrateCmd.Flags().Bool("overwrite", false, "Overwrite existing files in target")
	clawCmd.AddCommand(clawMigrateCmd)
}
