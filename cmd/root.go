package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "tedfetch",
	Short: "A CLI tool for downloading TED talk videos and subtitles",
	Long: `tedfetch is a command-line tool that helps you download TED talk videos and subtitles.
It supports downloading videos in different qualities and subtitles in various languages.`,
}

// Execute adds all child commands to the root command and sets flags appropriately.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
