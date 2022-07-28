package main

import (
	"bufio"
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/bootun/veronica/parser"
)

const (
	Version = "0.1.2"
)

var (
	projectPath string

	versionFlag bool

	changedFiles []string
)

func init() {
	flag.StringVar(&projectPath, "path", ".", "project path")
	flag.BoolVar(&versionFlag, "version", false, "print version")
}

// TODO: refactor
func main() {
	log.SetFlags(0)
	flag.Parse()
	if versionFlag {
		fmt.Printf("%s\n", Version)
		os.Exit(0)
	}
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
