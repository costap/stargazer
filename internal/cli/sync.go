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
	syncDryRun        bool
	syncNoAudio       bool
	syncKeepGoing     bool
	syncAllHandoffs   bool
	syncJSON          string
)

func init() {
	syncCmd.Flags().BoolVar(&syncAllHandoffs, "all-handoffs", false, "list every Accepted ADR in the hand-off section, including those marked dispatched in the state file")
	syncCmd.Flags().StringVar(&syncJSON, "json", "", "also emit the board as JSON: to stdout when given without a value (the terminal board is then suppressed), or to the file given as --json=path")
	syncCmd.Flags().Lookup("json").NoOptDefVal = "-"
	syncCmd.Flags().BoolVar(&syncDryRun, "dry-run", false, "reconcile and report, but write nothing: the markdown board and any --json=path file are left alone (--json on stdout still prints)")
	syncCmd.Flags().BoolVar(&syncNoAudio, "no-audio", false, "leave the off-desk audio queue (YouTube talks from signals and digests) out of the board")
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

		// JSON on stdout must be the only thing on stdout.
		if syncJSON == "-" {
			cfg.Dispatch.Silent = true
		}

		// 3. Initialize engine via package app
		engine := app.NewEngine(cfg)

		// 4. Run reconciliation workflow
		syncOpts := app.SyncOptions{
			DryRun:            syncDryRun,
			IncludeAudioQueue: !syncNoAudio,
			KeepGoing:         syncKeepGoing,
			AllHandoffs:       syncAllHandoffs,
			JSON:              syncJSON,
		}

		if err := engine.SyncWithOptions(ctx, syncOpts); err != nil {
			return fmt.Errorf("sync execution failed: %w", err)
		}

		return nil
	},
}
