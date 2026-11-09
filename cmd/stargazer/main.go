package main

import (
	"context"
	"os"

	"github.com/costap/stargazer/internal/cli"
	"github.com/joho/godotenv"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	cli.SetVersionInfo(version, commit, date)

	// Load .env if present; silently ignore if the file does not exist.
	_ = godotenv.Load()

	// Cobra has already printed the error; the process must still report it, or every
	// failure looks like success to scripts and to the tools that call this binary.
	if err := cli.Execute(context.Background()); err != nil {
		os.Exit(1)
	}
}
