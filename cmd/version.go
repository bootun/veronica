package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

const Version = "1.0.1"

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the version number of veronica",

	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("%s\n", Version)
	},
}
