package parser

import (
	"os"
	"regexp"

	"github.com/pkg/errors"
)

// Go mod 信息
type GoModuleInfo struct {
	// module xxx
	Name      string
	GoVersion string
	// ...
}

// ParseGoModuleInfo parse go.mod and return GoModuleInfo
func ParseGoModuleInfo(path string) (*GoModuleInfo, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return parseGoModContent(content)
}

func parseGoModContent(content []byte) (*GoModuleInfo, error) {
	var info GoModuleInfo
	// TODO: simple parser
	moduleExp := regexp.MustCompile(`module (.+)`)
	goVersionExp := regexp.MustCompile(`go (.+)`)

	if matches := moduleExp.FindSubmatch(content); len(matches) == 2 {
		info.Name = string(matches[1])
	} else {
		return nil, errors.New("module name not found")
	}

	if matches := goVersionExp.FindSubmatch(content); len(matches) == 2 {
		info.GoVersion = string(matches[1])
	} else {
		return nil, errors.New("go version not found")
	}
	return &info, nil
}
