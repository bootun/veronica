package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

const Version = "0.1.4"

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the version number of veronica",

	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("%s\n", Version)
	},
}
