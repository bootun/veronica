package astdiff

import (
	"fmt"
	"go/ast"
	"go/token"
	"log"
	"path/filepath"
	"reflect"
	"strings"

	"github.com/bootun/veronica/parser"
	"golang.org/x/tools/go/packages"
)

type ChangeType string

const (
	ChangeTypeAdded    ChangeType = "added"    // 新增
	ChangeTypeRemoved  ChangeType = "removed"  // 移除
	ChangeTypeModified ChangeType = "modified" // 修改
)

type Change struct {
	Type       ChangeType // "added", "removed", "modified"
	Package    string
	Object     string // 函数名、变量名等
	ObjectType string // "func", "var", "const", "type"
	ObjectID   string // 对象的唯一标识符
	File       string // 文件名
}

type AnalysisResult struct {
	Changes []Change
	Objects map[string]struct {
		Type     string
		Package  string
		Position token.Position
		Node     ast.Node
	}
}

func LoadDiff(oldPkgs, newPkgs []*packages.Package) (*AnalysisResult, error) {
	// 分析两个版本
	oldResult, err := analyzeCommit(oldPkgs)
	if err != nil {
		log.Fatalf("Failed to analyze old commit: %v", err)
	}
	newResult, err := analyzeCommit(newPkgs)
	if err != nil {
		log.Fatalf("Failed to analyze new commit: %v", err)
	}
	// 比较结果并输出
	result := compareResults(oldResult, newResult)
	return result, nil
}

func analyzeCommit(pkgs []*packages.Package) (*AnalysisResult, error) {
	// 分析包中的顶层定义
	result := &AnalysisResult{
		Objects: make(map[string]struct {
			Type     string
			Package  string
			Position token.Position
			Node     ast.Node
		}),
	}
	for _, pkg := range pkgs {
		analyzePackage(pkg, result)
	}

	return result, nil
}

func analyzePackage(pkg *packages.Package, result *AnalysisResult) {
	pkgName := pkg.ID
	for _, file := range pkg.Syntax {
		fullFileName := pkg.Fset.File(file.Pos()).Name()
		baseFileName := filepath.Base(fullFileName)

		// 用于存储当前遍历的顶层函数信息
		var currentFunc *ast.FuncDecl

		ast.Inspect(file, func(n ast.Node) bool {
			switch x := n.(type) {
			case *ast.FuncDecl:
				// 记录当前顶层函数
				currentFunc = x
				funcName := parser.GetFuncOrMethodName(x)
				id := parser.GetObjectID(pkgName, baseFileName, funcName)
				result.Objects[id] = struct {
					Type     string
					Package  string
					Position token.Position
					Node     ast.Node
				}{
					Type:     "func",
					Package:  pkgName,
					Position: pkg.Fset.Position(x.Pos()),
					Node:     x,
				}
			case *ast.GenDecl:
				// 如果这个声明在函数内部，使用顶层函数的信息
				if currentFunc != nil {
					funcName := parser.GetFuncOrMethodName(currentFunc)
					id := parser.GetObjectID(pkgName, baseFileName, funcName)

					// 如果这个ID已经存在，说明我们已经记录过这个函数了
					if _, exists := result.Objects[id]; exists {
						return true
					}

					result.Objects[id] = struct {
						Type     string
						Package  string
						Position token.Position
						Node     ast.Node
					}{
						Type:     "func",
						Package:  pkgName,
						Position: pkg.Fset.Position(currentFunc.Pos()),
						Node:     currentFunc,
					}
					return true
				}

				// 处理顶层声明
				for _, spec := range x.Specs {
					switch s := spec.(type) {
					case *ast.ValueSpec:
						for _, name := range s.Names {
							var objType string
							if x.Tok == token.CONST {
								objType = "const"
							} else {
								objType = "var"
							}
							id := parser.GetObjectID(pkgName, baseFileName, name.Name)
							result.Objects[id] = struct {
								Type     string
								Package  string
								Position token.Position
								Node     ast.Node
							}{
								Type:     objType,
								Package:  pkgName,
								Position: pkg.Fset.Position(name.Pos()),
								Node:     s,
							}
						}
					case *ast.TypeSpec:
						id := parser.GetObjectID(pkgName, baseFileName, s.Name.Name)
						result.Objects[id] = struct {
							Type     string
							Package  string
							Position token.Position
							Node     ast.Node
						}{
							Type:     "type",
							Package:  pkg.PkgPath,
							Position: pkg.Fset.Position(s.Pos()),
							Node:     s,
						}
					}
				}
			}
			return true
		})
	}
}

func compareResults(old, new *AnalysisResult) *AnalysisResult {
	result := &AnalysisResult{
		Objects: make(map[string]struct {
			Type     string
			Package  string
			Position token.Position
			Node     ast.Node
		}),
	}

	// 检查新增和修改的对象
	for key, newObj := range new.Objects {
		fullFileName := newObj.Position.Filename
		baseFileName := filepath.Base(fullFileName)
		parts := strings.Split(key, ":")
		objName := parts[len(parts)-1]
		fileName := fmt.Sprintf("%s/%s", newObj.Package, baseFileName)
		if oldObj, exists := old.Objects[key]; exists {
			// 检查包名和类型是否变化
			if oldObj.Package != newObj.Package || oldObj.Type != newObj.Type {
				result.Changes = append(result.Changes, Change{
					Type:       ChangeTypeModified,
					Package:    newObj.Package,
					Object:     objName,
					ObjectType: newObj.Type,
					ObjectID:   key,
					File:       fileName,
				})
			} else {
				// 检查对象内容是否变化
				if !astNodesEqual(oldObj.Node, newObj.Node) {
					result.Changes = append(result.Changes, Change{
						Type:       ChangeTypeModified,
						Package:    newObj.Package,
						Object:     objName,
						ObjectType: newObj.Type,
						ObjectID:   key,
						File:       fileName,
					})
				}
			}
		} else {
			parts := strings.Split(key, ":")
			objName := parts[len(parts)-1]
			result.Changes = append(result.Changes, Change{
				Type:       ChangeTypeAdded,
				Package:    newObj.Package,
				Object:     objName,
				ObjectType: newObj.Type,
				ObjectID:   key,
				File:       fileName,
			})
		}
	}

	// 检查删除的对象
	for key, oldObj := range old.Objects {
		if _, exists := new.Objects[key]; !exists {
			parts := strings.Split(key, ":")
			objName := parts[len(parts)-1]
			fullFileName := oldObj.Position.Filename
			baseFileName := filepath.Base(fullFileName)
			fileName := fmt.Sprintf("%s/%s", oldObj.Package, baseFileName)
			result.Changes = append(result.Changes, Change{
				Type:       ChangeTypeRemoved,
				Package:    oldObj.Package,
				Object:     objName,
				ObjectType: oldObj.Type,
				ObjectID:   key,
				File:       fileName,
			})
		}
	}

	return result
}

// astNodesEqual 比较两个AST节点是否相等
func astNodesEqual(a, b ast.Node) bool {
	// 都为 nil 则相等
	if a == nil && b == nil {
		return true
	}
	// 只有一个为 nil，则不相等。
	if a == nil || b == nil {
		return false
	}
	// 类型必须一致
	if reflect.TypeOf(a) != reflect.TypeOf(b) {
		return false
	}
	switch x := a.(type) {
	case *ast.File:
		y := b.(*ast.File)
		// 比较文件名
		if !astNodesEqual(x.Name, y.Name) {
			return false
		}
		// 比较所有声明
		if len(x.Decls) != len(y.Decls) {
			return false
		}
		for i := range x.Decls {
			if !astNodesEqual(x.Decls[i], y.Decls[i]) {
				return false
			}
		}
		return true
	case *ast.FuncDecl:
		y := b.(*ast.FuncDecl)
		// 比较接收者、名称、函数类型、函数体
		if !astNodesEqual(x.Recv, y.Recv) {
			return false
		}
		if !astNodesEqual(x.Name, y.Name) {
			return false
		}
		if !astNodesEqual(x.Type, y.Type) {
			return false
		}
		if !astNodesEqual(x.Body, y.Body) {
			return false
		}
		return true
	case *ast.FuncType:
		y := b.(*ast.FuncType)
		if !astNodesEqual(x.Params, y.Params) {
			return false
		}
		if !astNodesEqual(x.Results, y.Results) {
			return false
		}
		return true
	case *ast.FieldList:
		y := b.(*ast.FieldList)
		if x == nil || y == nil {
			return x == y
		}
		if len(x.List) != len(y.List) {
			return false
		}
		for i := range x.List {
			if !astNodesEqual(x.List[i], y.List[i]) {
				return false
			}
		}
		return true
	case *ast.Field:
		y := b.(*ast.Field)
		// 比较字段名列表
		if len(x.Names) != len(y.Names) {
			return false
		}
		for i := range x.Names {
			if !astNodesEqual(x.Names[i], y.Names[i]) {
				return false
			}
		}
		// 比较类型
		if !astNodesEqual(x.Type, y.Type) {
			return false
		}
		return true
	case *ast.Ident:
		y := b.(*ast.Ident)
		if x == nil || y == nil {
			return x == y
		}
		return x.Name == y.Name
	case *ast.BasicLit:
		y := b.(*ast.BasicLit)
		return x.Value == y.Value && x.Kind == y.Kind
	case *ast.BlockStmt:
		y := b.(*ast.BlockStmt)
		if len(x.List) != len(y.List) {
			return false
		}
		if len(x.List) != len(y.List) {
			return false
		}
		for i := range x.List {
			if !astNodesEqual(x.List[i], y.List[i]) {
				return false
			}
		}
		return true
	case *ast.ExprStmt:
		y := b.(*ast.ExprStmt)
		return astNodesEqual(x.X, y.X)
	case *ast.ReturnStmt:
		y := b.(*ast.ReturnStmt)
		if len(x.Results) != len(y.Results) {
			return false
		}
		for i := range x.Results {
			if !astNodesEqual(x.Results[i], y.Results[i]) {
				return false
			}
		}
		return true
	case *ast.BinaryExpr:
		y := b.(*ast.BinaryExpr)
		return x.Op == y.Op && astNodesEqual(x.X, y.X) && astNodesEqual(x.Y, y.Y)
	case *ast.CallExpr:
		y := b.(*ast.CallExpr)
		if !astNodesEqual(x.Fun, y.Fun) {
			return false
		}
		if len(x.Args) != len(y.Args) {
			return false
		}
		for i := range x.Args {
			if !astNodesEqual(x.Args[i], y.Args[i]) {
				return false
			}
		}
		return true
	case *ast.AssignStmt:
		y := b.(*ast.AssignStmt)
		if x.Tok != y.Tok || len(x.Lhs) != len(y.Lhs) || len(x.Rhs) != len(y.Rhs) {
			return false
		}
		for i := range x.Lhs {
			if !astNodesEqual(x.Lhs[i], y.Lhs[i]) {
				return false
			}
		}
		for i := range x.Rhs {
			if !astNodesEqual(x.Rhs[i], y.Rhs[i]) {
				return false
			}
		}
		return true
	case *ast.DeclStmt:
		y := b.(*ast.DeclStmt)
		return astNodesEqual(x.Decl, y.Decl)
	case *ast.IfStmt:
		y := b.(*ast.IfStmt)
		return astNodesEqual(x.Init, y.Init) &&
			astNodesEqual(x.Cond, y.Cond) &&
			astNodesEqual(x.Body, y.Body) &&
			astNodesEqual(x.Else, y.Else)
	case *ast.SelectorExpr:
		y := b.(*ast.SelectorExpr)
		return astNodesEqual(x.X, y.X) && astNodesEqual(x.Sel, y.Sel)
	case *ast.UnaryExpr:
		y := b.(*ast.UnaryExpr)
		return x.Op == y.Op && astNodesEqual(x.X, y.X)
	case *ast.CompositeLit:
		y := b.(*ast.CompositeLit)
		if !astNodesEqual(x.Type, y.Type) {
			return false
		}
		if len(x.Elts) != len(y.Elts) {
			return false
		}
		for i := range x.Elts {
			if !astNodesEqual(x.Elts[i], y.Elts[i]) {
				return false
			}
		}
		return true
	case *ast.StarExpr:
		y := b.(*ast.StarExpr)
		return astNodesEqual(x.X, y.X)
	case *ast.ParenExpr:
		y := b.(*ast.ParenExpr)
		return astNodesEqual(x.X, y.X)
	case *ast.IndexExpr:
		y := b.(*ast.IndexExpr)
		return astNodesEqual(x.X, y.X) && astNodesEqual(x.Index, y.Index)
	case *ast.SliceExpr:
		y := b.(*ast.SliceExpr)
		return astNodesEqual(x.X, y.X) && astNodesEqual(x.Low, y.Low) && astNodesEqual(x.High, y.High) && astNodesEqual(x.Max, y.Max)
	case *ast.KeyValueExpr:
		y := b.(*ast.KeyValueExpr)
		return astNodesEqual(x.Key, y.Key) && astNodesEqual(x.Value, y.Value)
	case *ast.MapType:
		y := b.(*ast.MapType)
		return astNodesEqual(x.Key, y.Key) && astNodesEqual(x.Value, y.Value)
	case *ast.ArrayType:
		y := b.(*ast.ArrayType)
		return astNodesEqual(x.Len, y.Len) && astNodesEqual(x.Elt, y.Elt)
	case *ast.StructType:
		y := b.(*ast.StructType)
		if len(x.Fields.List) != len(y.Fields.List) {
			return false
		}
		for i := range x.Fields.List {
			if !astNodesEqual(x.Fields.List[i], y.Fields.List[i]) {
				return false
			}
		}
		return true
	case *ast.InterfaceType:
		y := b.(*ast.InterfaceType)
		if len(x.Methods.List) != len(y.Methods.List) {
			return false
		}
		for i := range x.Methods.List {
			if !astNodesEqual(x.Methods.List[i], y.Methods.List[i]) {
				return false
			}
		}
		return true
	case *ast.Ellipsis:
		y := b.(*ast.Ellipsis)
		return astNodesEqual(x.Elt, y.Elt)
	case *ast.ChanType:
		y := b.(*ast.ChanType)
		return x.Dir == y.Dir && astNodesEqual(x.Value, y.Value)
	case *ast.ValueSpec:
		y := b.(*ast.ValueSpec)
		if len(x.Names) != len(y.Names) {
			return false
		}
		for i := range x.Names {
			if !astNodesEqual(x.Names[i], y.Names[i]) {
				return false
			}
		}
		if len(x.Values) != len(y.Values) {
			return false
		}
		for i := range x.Values {
			if !astNodesEqual(x.Values[i], y.Values[i]) {
				return false
			}
		}
		return astNodesEqual(x.Type, y.Type)
	case *ast.BadExpr:
		y := b.(*ast.BadExpr)
		return x.From == y.From
	case *ast.GenDecl:
		y := b.(*ast.GenDecl)
		if x.Tok != y.Tok || len(x.Specs) != len(y.Specs) {
			return false
		}
		if len(x.Specs) != len(y.Specs) {
			return false
		}
		for i := range x.Specs {
			if !astNodesEqual(x.Specs[i], y.Specs[i]) {
				return false
			}
		}
		return true
	case *ast.ImportSpec:
		y := b.(*ast.ImportSpec)
		return x.Path.Value == y.Path.Value && x.Name != nil && y.Name != nil && x.Name.Name == y.Name.Name
	case *ast.RangeStmt:
		y := b.(*ast.RangeStmt)
		return astNodesEqual(x.Key, y.Key) && astNodesEqual(x.Value, y.Value) && astNodesEqual(x.X, y.X) && astNodesEqual(x.Body, y.Body)
	case *ast.CaseClause:
		y := b.(*ast.CaseClause)
		if len(x.List) != len(y.List) {
			return false
		}
		for i := range x.List {
			if !astNodesEqual(x.List[i], y.List[i]) {
				return false
			}
		}
		return true
	case *ast.SwitchStmt:
		y := b.(*ast.SwitchStmt)
		return astNodesEqual(x.Init, y.Init) && astNodesEqual(x.Tag, y.Tag) && astNodesEqual(x.Body, y.Body)
	case *ast.TypeSpec:
		y := b.(*ast.TypeSpec)
		return astNodesEqual(x.Name, y.Name) && astNodesEqual(x.Type, y.Type)
	case *ast.TypeAssertExpr:
		y := b.(*ast.TypeAssertExpr)
		return astNodesEqual(x.X, y.X) && astNodesEqual(x.Type, y.Type)
	case *ast.FuncLit:
		y := b.(*ast.FuncLit)
		return astNodesEqual(x.Type, y.Type) && astNodesEqual(x.Body, y.Body)
	case *ast.DeferStmt:
		y := b.(*ast.DeferStmt)
		return astNodesEqual(x.Call, y.Call)
	case *ast.LabeledStmt:
		y := b.(*ast.LabeledStmt)
		return astNodesEqual(x.Label, y.Label) && astNodesEqual(x.Stmt, y.Stmt)
	case *ast.GoStmt:
		y := b.(*ast.GoStmt)
		return astNodesEqual(x.Call, y.Call)
	case *ast.SendStmt:
		y := b.(*ast.SendStmt)
		return astNodesEqual(x.Chan, y.Chan) && astNodesEqual(x.Value, y.Value)
	case *ast.IncDecStmt:
		y := b.(*ast.IncDecStmt)
		return astNodesEqual(x.X, y.X) && x.Tok == y.Tok
	case *ast.BranchStmt:
		y := b.(*ast.BranchStmt)
		return x.Tok == y.Tok && astNodesEqual(x.Label, y.Label)
	case *ast.BadStmt:
		y := b.(*ast.BadStmt)
		return x.From == y.From
	case *ast.ForStmt:
		y := b.(*ast.ForStmt)
		return astNodesEqual(x.Init, y.Init) && astNodesEqual(x.Cond, y.Cond) && astNodesEqual(x.Post, y.Post) && astNodesEqual(x.Body, y.Body)
	case *ast.SelectStmt:
		y := b.(*ast.SelectStmt)
		return astNodesEqual(x.Body, y.Body)
	case *ast.CommClause:
		y := b.(*ast.CommClause)
		if !astNodesEqual(x.Comm, y.Comm) {
			return false
		}
		if len(x.Body) != len(y.Body) {
			return false
		}
		for i := range x.Body {
			if !astNodesEqual(x.Body[i], y.Body[i]) {
				return false
			}
		}
		return true
	case *ast.TypeSwitchStmt:
		y := b.(*ast.TypeSwitchStmt)
		return astNodesEqual(x.Init, y.Init) && astNodesEqual(x.Assign, y.Assign) && astNodesEqual(x.Body, y.Body)
	default:
		panic(fmt.Sprintf("未处理的节点类型: %T, a: %v, b: %v\n", x, a, b))
	}
}
