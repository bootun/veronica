package parser

import (
	"fmt"
	"go/parser"
	"go/token"
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

	// initialize entrypoint
	entrypoint := make(map[string]*packageT)
	ignores := make(map[string][]string)
	hooks := make(map[string][]string)
	for _, v := range cfg.Services {
		fullRelPath := rootPath.Join(v.Entrypoint)
		relPath, err := fullRelPath.Rel(root)
		if err != nil {
			return nil, errors.WithMessage(err, "failed to get relative path")
		}
		entrypoint[relPath.String()] = &packageT{
			Name:       v.Name,
			Files:      make(map[string]*fileT),
			ImportedBy: make(map[string]*packageT),
			Imports:    make(map[string]*packageT),
		}
		ignores[relPath.String()] = v.Ignores
		hooks[relPath.String()] = v.Hooks
	}
	// initialize project
	project := &project{
		directory:     root,
		Module:        module,
		GoFileCounter: 0,
		Entrypoint:    entrypoint,
		parsed:        false,
		dependencies:  make(map[string][]string),
		Ignores:       ignores,
		Hooks:         hooks,
	}
	return project, nil
}

// project represents a monolithic go project, every entrypoint is a service
type project struct {
	// Module records the information of go.mod
	Module *GoModuleInfo
	// Entrypoint usually refers to main package of your services
	Entrypoint map[string]*packageT
	// number of go files
	GoFileCounter int // TODO: 改用map，统计所有文件

	// key is entrypoint package name, value is match pattern
	Ignores map[string][]string
	Hooks   map[string][]string

	parsed bool
	// key is package name, value is the dependent entrypoint
	dependencies map[string][]string
	// root directory of project
	directory string
}

// maybe a better name :-(
type packageT struct {
	// Name is package name, it only represents the path of
	// the current directory relative to the root directory.
	Name string
	// Files contains all files under the current package
	Files map[string]*fileT
	// Imports represent package dependencies, key is package
	// name
	Imports map[string]*packageT
	// ImportedBy means which packages are dependent on the
	// current package
	ImportedBy map[string]*packageT

	// NOTE: circular dependencies and different package names
	onStack bool
	walked  bool
}

func NewPackage(name string) *packageT {
	return &packageT{
		Name:       name,
		Files:      make(map[string]*fileT),
		Imports:    make(map[string]*packageT),
		ImportedBy: make(map[string]*packageT),
	}
}

func (p *packageT) AddFile(file *fileT) error {
	if file == nil {
		return errors.New("file is empty")
	}
	p.Files[file.Name] = file
	file.Package = p
	return nil
}

func (p *packageT) Import(pkg *packageT) error {
	if pkg == nil {
		return errors.New("package is empty")
	}
	p.Imports[pkg.Name] = pkg
	pkg.ImportedBy[p.Name] = p
	return nil
}

type fileT struct {
	// Name is the relative path to the root directory
	Name string
	// Package indicates the package to which the file belongs
	Package *packageT
	// Imports represent package dependencies, key is package name
	Imports map[string]*packageT
}

func NewFile(name string) *fileT {
	return &fileT{
		Name:    name,
		Package: nil,
		Imports: make(map[string]*packageT),
	}
}

func (p *project) IsParsed() bool {
	return p.parsed
}

// ParseProject parse project dependencies
func (p *project) Parse() error {
	projectPath := path.New(p.directory)
	if !projectPath.IsDir() {
		return errors.Errorf("%s is not a directory", projectPath.String())
	}

	// all parsed packages, including which files are under the package, key is package name
	var parsedPkg = make(map[string]*packageT)

	fset := token.NewFileSet()

	err := projectPath.Walk(func(curFilePath path.FilePath) error {
		if !curFilePath.IsDir() && curFilePath.HasExt(".go") {
			// increment counter
			p.GoFileCounter++

			file, err := parser.ParseFile(fset, curFilePath.String(), nil, parser.ImportsOnly)
			if err != nil {
				return err
			}

			// relative path of file
			relCurFile, err := projectPath.Rel(curFilePath.String())
			if err != nil {
				return err
			}
			// relative name of current package
			relCurPkg := curFilePath.Dir()

			curFile := NewFile(relCurFile.String())
			// create package if not exists
			if p, exists := parsedPkg[relCurPkg.String()]; !exists {
				pkg := NewPackage(relCurPkg.String())
				_ = pkg.AddFile(curFile)
				parsedPkg[relCurPkg.String()] = pkg
			} else {
				p.AddFile(curFile)
			}

			// parse imports
			for _, importSpec := range file.Imports {
				if !strings.Contains(importSpec.Path.Value, p.Module.Name) {
					// skip standard library and external dependencies
					continue
				}
				// relative import package
				relCurImport := strings.TrimPrefix(
					strings.Trim(importSpec.Path.Value, `"`),
					p.Module.Name+"/",
				)

				// process current import package
				if importPkg, parsed := parsedPkg[relCurImport]; parsed {
					_ = parsedPkg[relCurPkg.String()].Import(importPkg)
				} else {
					importPkg := NewPackage(relCurImport)
					parsedPkg[relCurImport] = importPkg
					_ = parsedPkg[relCurPkg.String()].Import(importPkg)
				}

			}
		}
		return nil
	})
	if err != nil {
		return err
	}

	for k, _ := range p.Entrypoint {
		if pkg, exists := parsedPkg[k]; exists {
			p.Entrypoint[k] = pkg
		} else {
			return errors.Errorf("entrypoint %s not exists", k)
		}
	}
	p.walkEntryPoint()
	p.parsed = true
	return nil
}

func (p *project) walkEntryPoint() {
	for entrypoint, pkg := range p.Entrypoint {
		// key is package name, value is the dependent entrypoint
		dependencies := make(map[string]*packageT)
		WalkPackageDependencies(pkg, dependencies)
		for k, _ := range dependencies {
			p.dependencies[k] = append(p.dependencies[k], entrypoint)
		}
	}
}

func WalkPackageDependencies(pkg *packageT, dependencies map[string]*packageT) {
	if pkg.onStack {
		// FIXME: 环
		return
	}

	pkg.walked = true
	pkg.onStack = true
	for _, p := range pkg.Imports {
		if _, walked := dependencies[p.Name]; !walked {
			dependencies[p.Name] = p
		}
		WalkPackageDependencies(p, dependencies)
	}
	pkg.onStack = false
}

func (p *project) ReportImpact(changed []string) {
	if !p.parsed {
		fmt.Printf("project not parsed\n")
		return
	}
	for _, file := range changed {
		fileDir := path.New(file).Dir()
		if entrypoints, exists := p.dependencies[fileDir.String()]; exists {
			fmt.Printf("改动了 %s 包中的 %s 文件,可能会影响这些包的构建:\n", fileDir, file)
			for _, pkg := range entrypoints {
				fmt.Printf("    - %s", pkg)
			}
			fmt.Println()
		}
	}
}

// GetAffectedEntrypoint returns affected entrypoint, each entrypoint can only appear once at most.
func (p *project) GetAffectedEntrypoint(changed []string) ([]string, error) {
	if !p.parsed {
		return nil, errors.New("project not parsed")
	}

	// record entrypoint that have been processed
	processed := make(map[string]struct{}, len(p.Entrypoint))
	var result []string
	for _, file := range changed {
		pFile := path.New(file)

		filePkg := pFile.Dir()
		for k, _ := range p.Entrypoint {
			hooks := p.Hooks[k]
			for _, hook := range hooks {
				if pFile.Match(hook) {
					if _, exists := processed[k]; !exists {
						result = append(result, k)
						processed[k] = struct{}{}
					}
				}
			}
		}
		// if the file affects entrypoint, add it to result
		if entrypoints, exists := p.dependencies[filePkg.String()]; exists {
		affected:
			// record affected entrypoint
			for _, point := range entrypoints {
				ignores := p.Ignores[point]
				for _, ignore := range ignores {
					if pFile.Match(ignore) {
						continue affected
					}
				}

				// if the entrypoint has been processed, skip it
				if _, exists := processed[point]; !exists {
					result = append(result, point)
					processed[point] = struct{}{}
				}
			}

		}
	}
	return result, nil
}
