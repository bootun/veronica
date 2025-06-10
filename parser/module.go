package parser

import (
	"os"

	"github.com/pkg/errors"
	"golang.org/x/mod/modfile"
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
	mf, err := modfile.Parse("go.mod", content, nil)
	if err != nil {
		return nil, err
	}
	if mf.Module == nil {
		return nil, errors.New("module name not found")
	}
	if mf.Go == nil {
		return nil, errors.New("go version not found")
	}
	info := &GoModuleInfo{
		Name:      mf.Module.Mod.Path,
		GoVersion: mf.Go.Version,
	}
	return info, nil
}
