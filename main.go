package main

import (
	"bufio"
	"flag"
	"log"
	"os"

	"github.com/bootun/veronica/parser"
)

var (
	projectPath string

	changedFiles []string
)

func init() {
	flag.StringVar(&projectPath, "path", ".", "project path")
}

// TODO: refactor
func main() {
	log.SetFlags(0)
	flag.Parse()

	project, err := parser.NewProject(projectPath)
	if err != nil {
		log.Fatal(err)
	}
	if err := project.Parse(); err != nil {
		log.Fatal(err)
	}

	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		changedFiles = append(changedFiles, scanner.Text())
	}
	entrypoints, err := project.GetAffectedEntrypoint(changedFiles)
	if err != nil {
		log.Fatal(err)
	}
	for _, v := range entrypoints {
		log.Printf("%s", v)
	}
	// project.ReportImpact(changedFiles)
}
