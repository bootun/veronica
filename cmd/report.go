package cmd

import (
	"fmt"
	"log"
	"os/exec"
	"strings"

	"github.com/bootun/veronica/parser"
	"github.com/spf13/cobra"
)

var reportCmd = &cobra.Command{
	Use:     "report",
	Example: `  veronica report --old HEAD~1 --new HEAD`,
	Short:   "report the scope of impact of code changes",
	Run: func(command *cobra.Command, args []string) {
		if oldCommit == "" {
			log.Fatal("Usage: veronica report --old <old-commit> --new <new-commit>")
		}
		if newCommit == "" {
			log.Fatal("Usage: veronica report --old <old-commit> --new <new-commit>")
		}

		cmd := exec.Command("git", "diff", "--name-only", oldCommit, newCommit)
		output, err := cmd.CombinedOutput()
		if err != nil {
			log.Fatalf("git diff: %s", output)
		}

		project, err := parser.NewProject(repo)
		if err != nil {
			log.Fatalf("load project: %s", err)
		}
		if err := project.Parse(); err != nil {
			log.Fatalf("parse project: %s", err)
		}
		changedFiles := strings.Split(string(output), "\n")
		entrypoints, err := project.GetAffectedEntrypoint(changedFiles)
		if err != nil {
			log.Fatalf("get affected entrypoint: %s", err)
		}
		switch outputFormat {
		case OutputFormatOneLine:
			for _, v := range entrypoints {
				fmt.Printf("%s\n", v)
			}
		case OutputFormatText:
			project.ReportImpact(changedFiles)
		default:
			log.Fatalf("unknown output format: %s", outputFormat)
		}
	},
}

var (
	outputFormat string
	oldCommit    string
	newCommit    string
	repo         string
)

const (
	OutputFormatOneLine = "oneline"
	OutputFormatText    = "text"
)

func init() {
	reportCmd.Flags().StringVarP(&outputFormat, "format", "f", OutputFormatOneLine, "output format, options: oneline, text")
	reportCmd.Flags().StringVarP(&oldCommit, "old", "o", "", "old commit")
	reportCmd.Flags().StringVarP(&newCommit, "new", "n", "", "new commit")
	reportCmd.Flags().StringVarP(&repo, "repo", "r", ".", "repo path")
}
