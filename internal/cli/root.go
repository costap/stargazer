package cli

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/costap/stargazer/internal/config"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	cfgFile string
	cfg     *config.Config
	v       *viper.Viper
	rootCmd = &cobra.Command{
		Use:   "stargazer",
		Short: "Stargazer coordinates architecture roadmap phases, ADRs, and Astrometrics radar signals",
		Long: `Stargazer coordinates distributed architecture fronts across
platform, networking, and domain services. It scans multi-group ADR directories,
ingests Astrometrics radar signals, and syncs execution states into ARCH_DISPATCH.md.`,
	}
)

func Execute(ctx context.Context) error {
	return rootCmd.ExecuteContext(ctx)
}

func init() {
	cobra.OnInitialize(initConfig)

	v = viper.New()

	// Persistent Flags
	rootCmd.PersistentFlags().StringVarP(&cfgFile, "config", "c", "", "config file (default is .stargazer.yaml)")
	rootCmd.PersistentFlags().String("roadmap", "roadmap.yaml", "path to product roadmap YAML")
	rootCmd.PersistentFlags().String("signals", "", "URL or File to fetch signals JSON")
	rootCmd.PersistentFlags().StringP("out", "o", "ARCH_DISPATCH.md", "target dispatch markdown output file")
	rootCmd.PersistentFlags().Int("wip-limit", 3, "WIP limit for proposed ADRs across all groups")
	rootCmd.PersistentFlags().Bool("silent", false, "suppress terminal summary output")

	// Bind flags to Viper
	_ = v.BindPFlag("roadmap.file", rootCmd.PersistentFlags().Lookup("roadmap"))
	_ = v.BindPFlag("signals.source", rootCmd.PersistentFlags().Lookup("signals"))
	_ = v.BindPFlag("dispatch.output", rootCmd.PersistentFlags().Lookup("out"))
	_ = v.BindPFlag("adr.wip_limit", rootCmd.PersistentFlags().Lookup("wip-limit"))
	_ = v.BindPFlag("dispatch.silent", rootCmd.PersistentFlags().Lookup("silent"))

	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(syncCmd)
}

func initConfig() {
	// Set Default Values
	v.SetDefault("roadmap.file", "roadmap.yaml")
	v.SetDefault("dispatch.output", "ARCH_DISPATCH.md")
	v.SetDefault("adr.wip_limit", 3)
	v.SetDefault("dispatch.silent", false)
	v.SetDefault("signals.filter_rings", []string{"assess", "trial", "explore"})
	v.SetDefault("signals.filter_tags", []string{"poc", "explore"})

	// Config File Discovery
	if cfgFile != "" {
		v.SetConfigFile(cfgFile)
	} else {
		v.SetConfigName(".stargazer")
		v.SetConfigType("yaml")
		v.AddConfigPath(".")
		v.AddConfigPath("$HOME/.config/stargazer")
		v.AddConfigPath("$HOME")
	}

	// Environment Variables: STARGAZER_ADR_WIP_LIMIT, STARGAZER_ROADMAP_FILE, etc.
	v.SetEnvPrefix("STARGAZER")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_", "-", "_"))
	v.AutomaticEnv()

	if err := v.ReadInConfig(); err == nil {
		// Config file found and loaded
	} else {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			fmt.Fprintf(os.Stderr, "Warning reading config file: %v\n", err)
		}
	}
}

// GetViper returns the configured Viper instance for subcommands
func GetViper() *viper.Viper {
	return v
}
