package parser

import (
	"strings"

	"github.com/pkg/errors"

	"github.com/bootun/veronica/config"
	"github.com/bootun/veronica/tools/path"
)

// NewProject create a new project
func NewProject(root string) (*project, error) {
	rootPath := path.New(root)
	if !rootPath.IsDir() {
		return nil, errors.Errorf("%s is not a directory", root)
	}

	// parse veronica config
	var configPath string
	yml, yaml := rootPath.Join("veronica.yml"), rootPath.Join("veronica.yaml")
	if yaml.IsFile() {
		configPath = yaml.String()
	} else if yml.IsFile() {
		configPath = yml.String()
	} else {
		return nil, errors.New("veronica config file not found")
	}
	cfg, err := config.New(configPath)
	if err != nil {
		return nil, errors.WithMessage(err, "failed to parse veronica config file")
	}

	// parse go.mod
	var gomodPath string
	if cfg.GoMod == "" {
		gomodPath = rootPath.Join("go.mod").String()
	} else {
		gomodPath = cfg.GoMod
	}
	module, err := ParseGoModuleInfo(gomodPath)
	if err != nil {
		return nil, errors.WithMessage(err, "failed to parse go.mod")
	}
	if module == nil {
		return nil, errors.New("invalid go.mod file")
	}
	moduleName := module.Name
	if moduleName == "" {
		return nil, errors.New("invalid go.mod file, module name is empty")
	}
	services := make(map[string]Service)
	// initialize entrypoint
	ignores := make(map[string][]string)
	hooks := make(map[string][]string)
	for _, v := range cfg.Services {
		entrypoint := v.Entrypoint
		if !strings.HasPrefix(entrypoint, moduleName) {
			entrypoint = moduleName + "/" + entrypoint
		}
		fullRelPath := rootPath.Join(entrypoint)
		relPath, err := fullRelPath.Rel(root)
		if err != nil {
			return nil, errors.WithMessage(err, "failed to get relative path")
		}
		services[entrypoint] = Service{
			Name:       v.Name,
			Entrypoint: entrypoint,
			Ignores:    v.Ignores,
			Hooks:      v.Hooks,
		}
		ignores[relPath.String()] = v.Ignores
		hooks[relPath.String()] = v.Hooks
	}
	// initialize project
	project := &project{
		directory: root,
		Module:    module,
		Services:  services,
		Ignores:   ignores,
		Hooks:     hooks,
	}
	return project, nil
}

// project represents a monolithic go project, every entrypoint is a service
type project struct {
	// Module records the information of go.mod
	Module *GoModuleInfo
	// key: service entrypoint, value: service info
	Services map[string]Service

	// key is entrypoint package name, value is match pattern
	Ignores map[string][]string
	Hooks   map[string][]string

	// root directory of project
	directory string
}

type Service struct {
	Name       string
	Entrypoint string
	Ignores    []string
	Hooks      []string
}
