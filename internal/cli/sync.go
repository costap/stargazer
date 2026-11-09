package cli

import (
	"fmt"

	"github.com/costap/stargazer/internal/app"
	"github.com/costap/stargazer/internal/config"
	"github.com/spf13/cobra"
)

var (
	syncRoadmapPath   string
	syncSignalsSource string
	syncOutputPath    string
	syncWIPLimit      int
	syncDryRun        bool
	syncAudioQueue    bool
	syncKeepGoing     bool
)

func init() {
	syncCmd.Flags().BoolVar(&syncKeepGoing, "keep-going", false, "render a degraded board when an input cannot be read, instead of failing")
}

// flagKeys maps the flags that override a config value to the Viper key they set.
var flagKeys = map[string]string{
	"roadmap": "roadmap.file",
	"out":     "dispatch.output",
	"signals": "signals.source",
}

var syncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Reconcile ADR directories, roadmap phases, and Astrometrics signals into ARCH_DISPATCH.md",
	Long: `Sync scans all configured ADR tracks (e.g. 2-product, 3-platform),
evaluates open needs and unassigned decisions in roadmap.yaml, ingests
signals.json (including conference talk queues), and outputs a regenerated ARCH_DISPATCH.md.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()
		v := GetViper()
		cmd.SilenceUsage = true // a runtime failure is not a usage error

		// 1. Resolve configuration through Viper (flag > env > config file > default)
		var overridden []string
		for flag, key := range flagKeys {
			if cmd.Flags().Changed(flag) {
				overridden = append(overridden, key)
			}
		}
		cfg, err := config.Load(v, overridden...)
		if err != nil {
			return fmt.Errorf("configuration error: %w", err)
		}

		// 2. Apply any command-specific flag overrides
		if syncRoadmapPath != "" {
			cfg.Roadmap.File = syncRoadmapPath
		}
		if syncSignalsSource != "" {
			cfg.Signals.Source = syncSignalsSource
		}
		if cmd.Flags().Changed("wip-limit") {
			cfg.ADR.WIPLimit = syncWIPLimit
		}

		// 3. Initialize engine via package app
		engine := app.NewEngine(cfg)

		// 4. Run reconciliation workflow
		syncOpts := app.SyncOptions{
			DryRun:            syncDryRun,
			IncludeAudioQueue: syncAudioQueue,
			KeepGoing:         syncKeepGoing,
		}

		if err := engine.SyncWithOptions(ctx, syncOpts); err != nil {
			return fmt.Errorf("sync execution failed: %w", err)
		}

		return nil
	},
}
