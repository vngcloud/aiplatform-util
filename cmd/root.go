package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var (
	version = "0.1.0"
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "aiplatform-util",
	Short: "A CLI tool for managing network volumes in AI Platform notebook environments",
	Long: `aiplatform-util helps data scientists and ML engineers manage their network volumes
(S3-compatible storage) in AI Platform notebook environments.

It provides a git-like interface for listing, pulling, and pushing files between
your local workspace and the network volume.`,
	Version: version,
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	// Global flags can be added here
	// rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.aiplatform-util.yaml)")
}
