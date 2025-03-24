package cmd

import "github.com/spf13/cobra"

var rootCmd = &cobra.Command{
	Use:   "veronica",
	Short: "a tool for reporting the scope of impact of code changes",
	Long:  `veronica is a tool for reporting the scope of impact of code changes`,
}

func init() {
	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(dependencyCmd)
	rootCmd.AddCommand(impactCmd)
}

func Execute() error {
	return rootCmd.Execute()
}
