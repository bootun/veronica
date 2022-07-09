package main

import (
	"bufio"
	"flag"
	"log"
	"os"

	"github.com/bootun/veronica/parser"
)

var (
	// 项目根目录
	projectPath string

	// 从标准输入读取文件
	inputMode bool

	changedFiles []string
)

func init() {
	flag.StringVar(&projectPath, "path", ".", "project path")
	flag.BoolVar(&inputMode, "input", false, "input from stdin")
}

// TODO: refactor
func main() {
	log.SetFlags(0)
	flag.Parse()
	
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		changedFiles = append(changedFiles, scanner.Text())
	}
	
	project, err := parser.NewProject(projectPath)
	if err != nil {
		log.Fatal(err)
	}
	if err := project.Parse(); err != nil {
		log.Fatal(err)
	}
	project.ReportImpact(changedFiles)
}
