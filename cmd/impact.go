package cmd

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/bootun/veronica/astdiff"
	"github.com/bootun/veronica/parser"
	"github.com/spf13/cobra"
)

var impactCmd = &cobra.Command{
	Use:   "impact",
	Short: "impact",
	Run: func(cmd *cobra.Command, args []string) {
		if oldCommit == "" {
			cmd.Usage()
			os.Exit(1)
		}
		if newCommit == "" {
			cmd.Usage()
			os.Exit(1)
		}
		Impact(oldCommit, newCommit)
	},
}

var (
	oldCommit    string
	newCommit    string
	repo         string // 仓库路径
	scope        string // 报告的变更范围(all, service)
)

const (
	ScopeAll     = "all"
	ScopeService = "service"
)

func init() {
	impactCmd.Flags().StringVarP(&oldCommit, "old", "o", "", "old commit")
	impactCmd.Flags().StringVarP(&newCommit, "new", "n", "", "new commit")
	impactCmd.Flags().StringVarP(&repo, "repo", "r", ".", "repo path")
	impactCmd.Flags().StringVarP(&scope, "scope", "s", ScopeAll, "report scope, options: all, service")
}

func Impact(oldCommit, newCommit string) {
	log.SetFlags(log.Lshortfile | log.LstdFlags)
	project, err := parser.NewProject(repo)
	if err != nil {
		log.Fatalf("load project: %s", err)
	}
	// 创建临时目录
	tmpDir, err := os.MkdirTemp("", "veronica-astdiff-*")
	if err != nil {
		log.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tmpDir)
	oldDir := filepath.Join(tmpDir, "old")
	newDir := filepath.Join(tmpDir, "new")
	// 使用 git archive 导出指定版本
	if err := exportCommit(oldCommit, oldDir); err != nil {
		log.Fatalf("failed to export commit: %v", err)
	}
	if err := exportCommit(newCommit, newDir); err != nil {
		log.Fatalf("failed to export commit: %v", err)
	}

	// 加载包信息
	oldPkgs, err := parser.LoadPackages(oldDir)
	if err != nil {
		log.Fatalf("failed to load packages: %v", err)
	}
	newPkgs, err := parser.LoadPackages(newDir)
	if err != nil {
		log.Fatalf("failed to load packages: %v", err)
	}

	// 分析AST差异
	diff, err := astdiff.LoadDiff(oldPkgs, newPkgs)
	if err != nil {
		log.Fatalf("failed to load diff: %v", err)
	}
	oldDeps, err := parser.BuildDependency(oldPkgs)
	if err != nil {
		log.Fatalf("failed to build dependency: %v", err)
	}
	newDeps, err := parser.BuildDependency(newPkgs)
	if err != nil {
		log.Fatalf("failed to build dependency: %v", err)
	}

	switch scope {
	case ScopeAll:
		// 报告所有影响
		for _, change := range diff.Changes {
			switch change.Type {
			case astdiff.ChangeTypeAdded:
				deps, err := newDeps.GetDependency(change.ObjectID)
				if err != nil {
					log.Fatalf("failed to get dependency: %v", err)
				}
				fmt.Printf("add %s in %s, dependencies:\n", change.Object, change.File)
				for i, dep := range deps {
					fmt.Printf("  %d. %s\n", i+1, dep)
				}
			case astdiff.ChangeTypeRemoved:
				deps, err := oldDeps.GetDependency(change.ObjectID)
				if err != nil {
					log.Fatalf("failed to get dependency: %v", err)
				}
				fmt.Printf("remove %s in %s, dependencies:\n", change.Object, change.File)
				for i, dep := range deps {
					fmt.Printf("  %d. %s\n", i+1, dep)
				}
			case astdiff.ChangeTypeModified:
				deps, err := newDeps.GetDependency(change.ObjectID)
				if err != nil {
					log.Fatalf("failed to get dependency: %v", err)
				}
				fmt.Printf("modify %s in %s, dependencies:\n", change.Object, change.File)
				for i, dep := range deps {
					fmt.Printf("  %d. %s\n", i+1, dep)
				}
			}
		}
	case ScopeService:
		// 只报告受影响的服务
		effectedServices := getEffectedServices(project.Services, oldDeps, newDeps, diff.Changes)
		// fmt.Printf("受影响的entrypoint有:\n")
		for _, service := range effectedServices {
			fmt.Printf("%s\n", service)
		}
	default:
		log.Fatalf("invalid scope: %s", scope)
	}
}

func exportCommit(commit, dir string) error {
	// 确保目标目录存在
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory %s: %v", dir, err)
	}

	// 使用 git archive 导出指定版本
	cmd := exec.Command("git", "archive", "--format=tar", commit)
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("failed to archive commit: %v", err)
	}

	// 解压到临时目录
	cmd = exec.Command("tar", "-xf", "-")
	cmd.Dir = dir
	cmd.Stdin = strings.NewReader(string(output))
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to extract archive: %v", err)
	}

	return nil
}

func getEffectedServices(services map[string]parser.Service, oldDeps, newDeps *parser.DependencyInfo, changes []astdiff.Change) []string {
	effecteds := make(map[string]bool)
	for _, change := range changes {
		switch change.Type {
		case astdiff.ChangeTypeAdded:
			deps, err := newDeps.GetDependency(change.ObjectID)
			if err != nil {
				log.Fatalf("failed to get dependency: %v", err)
			}
			for _, dep := range deps {
				effecteds[dep] = true
			}
		case astdiff.ChangeTypeRemoved:
			deps, err := oldDeps.GetDependency(change.ObjectID)
			if err != nil {
				log.Fatalf("failed to get dependency: %v", err)
			}
			for _, dep := range deps {
				effecteds[dep] = true
			}
		case astdiff.ChangeTypeModified:
			deps, err := newDeps.GetDependency(change.ObjectID)
			if err != nil {
				log.Fatalf("failed to get dependency: %v", err)
			}
			for _, dep := range deps {
				effecteds[dep] = true
			}
		}
	}
	effectedServices := make([]string, 0, len(effecteds))
	for effected := range effecteds {
		if svc, ok := services[effected]; ok {
			effectedServices = append(effectedServices, svc.Name)
		}
	}
	return effectedServices
}
