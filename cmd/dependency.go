package cmd

import (
	"fmt"
	"log"
	"os"

	"github.com/bootun/veronica/parser"
	"github.com/spf13/cobra"
)

var dependencyCmd = &cobra.Command{
	Use:   "dependency",
	Short: "object dependency analysis",
	Run: func(cmd *cobra.Command, args []string) {
		if targetID == "" {
			cmd.Usage()
			os.Exit(1)
		}
		pkgs, err := parser.LoadPackages(repo)
		if err != nil {
			log.Fatalf("load packages: %s", err)
		}
		dependencyInfo, err := parser.BuildDependency(pkgs)
		if err != nil {
			log.Fatalf("build dependency: %s", err)
		}
		deps, err := dependencyInfo.GetDependency(targetID)
		if err != nil {
			log.Fatalf("get dependency: %s", err)
		}
		fmt.Printf("target: %s\n", targetID)
		for _, dep := range deps {
			fmt.Println(dep)
		}
	},
}

var (
	targetID string
)

func init() {
	dependencyCmd.Flags().StringVarP(&targetID, "target", "t", "", "target")
	dependencyCmd.Flags().StringVarP(&repo, "repo", "r", ".", "repo path")
}
