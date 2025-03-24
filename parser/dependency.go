package parser

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"path/filepath"
	"strings"

	"golang.org/x/tools/go/packages"
)

// node 表示一个顶级声明节点，使用"文件:标识符"作为唯一标识。
type node struct {
	Pos  token.Pos
	File string // 文件名（仅基础名）
	Name string // 标识符名称
	Obj  types.Object
}

// interfaceInfo 存储接口相关信息
type interfaceInfo struct {
	// key: 方法名称, value: 方法签名
	Methods map[string]*types.Func
}

// interfaceImplementations 存储接口和实现类型的映射关系
type interfaceImplementations struct {
	// 接口ID -> 实现该接口的类型ID列表
	ImplementersMap map[string][]string
	// 接口ID -> 接口方法名称 -> 实现该方法的类型ID列表
	MethodImplementersMap map[string]map[string][]string
}

// Graph 存储节点之间的依赖关系，边表示"当前节点依赖于另一个节点"
type Graph map[string]map[string]struct{}

// GetObjectID 获取完整标识符路径，格式：包名/文件名:标识符
// pkg应为包括go module name的完整包名，例如：github.com/bootun/veronica/parser
func GetObjectID(pkg string, fileName string, obj string) string {
	if pkg == "" {
		panic("pkg is empty")
	}
	if fileName == "" {
		panic("fileName is empty")
	}
	if obj == "" {
		panic("obj is empty")
	}
	return fmt.Sprintf("%s/%s:%s", pkg, fileName, obj)
}

type DependencyInfo struct {
	// 项目内所有的顶级声明, key: NodeID, value: Node
	nodes map[string]*node
	// 依赖图的反向图, key: 依赖的节点ID, value: 集合（set）内存放被依赖的节点ID
	revGraph Graph
}

// GetDependency 获取 targetID 的依赖节点
func (d *DependencyInfo) GetDependency(targetID string) ([]string, error) {
	if _, ok := d.nodes[targetID]; !ok {
		return nil, fmt.Errorf("target %s is not defined in project", targetID)
	}

	// 利用深度优先搜索（DFS）查找所有直接或间接依赖 targetID 的节点
	visited := make(map[string]struct{})
	var dfs func(string)
	dfs = func(node string) {
		for dep, _ := range d.revGraph[node] {
			if _, ok := visited[dep]; !ok {
				visited[dep] = struct{}{}
				dfs(dep)
			}
		}
	}
	dfs(targetID)

	if len(visited) == 0 {
		return []string{}, nil
	}

	deps := make([]string, 0, len(visited))
	for id := range visited {
		deps = append(deps, id)
	}
	return deps, nil
}

// BuildDependency 构建依赖关系图
func BuildDependency(repoRoot string) (*DependencyInfo, error) {
	// 加载包信息
	cfg := &packages.Config{
		Mode: packages.NeedName | packages.NeedFiles | packages.NeedSyntax | packages.NeedTypes | packages.NeedTypesInfo,
		Dir:  repoRoot,
	}
	pkgs, err := packages.Load(cfg, "./...")
	if err != nil {
		return nil, fmt.Errorf("parser project AST failed in %s: %v", repoRoot, err)
	}

	// nodesMap：key: 对象, value: 节点唯一标识
	// 项目内所有的顶级声明
	nodesMap := make(map[types.Object]string)
	// nodesInfo：存储每个节点的信息
	// key: NodeID, value: Node
	nodesInfo := make(map[string]*node)

	// 所有接口信息
	// key: 接口的fullName表示, value: 接口信息
	interfacesInfo := make(map[string]*interfaceInfo)

	// 接口与实现类型的映射关系
	interfaceImpls := &interfaceImplementations{
		ImplementersMap:       make(map[string][]string),
		MethodImplementersMap: make(map[string]map[string][]string),
	}

	// 依赖图: key->当前节点ID, value->集合（set）内存放依赖的节点ID
	graph := make(Graph)

	// 遍历所有包和文件，提取顶级声明，构建接口表
	for _, pkg := range pkgs {
		fset := pkg.Fset
		for _, file := range pkg.Syntax {
			fullFilename := fset.File(file.Pos()).Name() // 文件的绝对路径
			baseFilename := filepath.Base(fullFilename)  // 文件名
			// 遍历文件中的所有顶级声明
			for _, decl := range file.Decls {
				switch d := decl.(type) {
				case *ast.FuncDecl:
					// 函数或者方法
					if d.Name == nil {
						continue
					}
					funcName := GetFuncOrMethodName(d)
					// 节点唯一标识
					id := GetObjectID(pkg.ID, baseFilename, funcName)
					obj := pkg.TypesInfo.Defs[d.Name]
					if obj == nil {
						continue
					}
					nodesMap[obj] = id
					nodesInfo[id] = &node{
						Pos:  d.Pos(),
						File: baseFilename,
						Name: funcName,
						Obj:  obj,
					}
					// 初始化依赖图节点
					if _, ok := graph[id]; !ok {
						graph[id] = make(map[string]struct{})
					}
				case *ast.GenDecl:
					// 变量、常量、类型定义等
					for _, spec := range d.Specs {
						switch s := spec.(type) {
						case *ast.ValueSpec:
							// 可能是变量或常量定义，可能有多个名字
							for _, ident := range s.Names {
								if ident == nil {
									continue
								}
								id := GetObjectID(pkg.ID, baseFilename, ident.Name)
								obj := pkg.TypesInfo.Defs[ident]
								if obj == nil {
									continue
								}
								nodesMap[obj] = id
								nodesInfo[id] = &node{
									Pos:  ident.Pos(),
									File: baseFilename,
									Name: ident.Name,
									Obj:  obj,
								}
								if _, ok := graph[id]; !ok {
									graph[id] = make(map[string]struct{})
								}
							}
						case *ast.TypeSpec:
							// 类型定义
							if s.Name == nil {
								continue
							}
							id := GetObjectID(pkg.ID, baseFilename, s.Name.Name)
							obj := pkg.TypesInfo.Defs[s.Name]
							if obj == nil {
								continue
							}
							nodesMap[obj] = id
							nodesInfo[id] = &node{
								Pos:  s.Pos(),
								File: baseFilename,
								Name: s.Name.Name,
								Obj:  obj,
							}
							if _, ok := graph[id]; !ok {
								graph[id] = make(map[string]struct{})
							}

							// 检查是否为接口定义
							if t, ok := s.Type.(*ast.InterfaceType); ok {
								// 记录接口信息
								iface := &interfaceInfo{
									Methods: make(map[string]*types.Func),
								}

								// 提取接口方法
								if t.Methods != nil {
									for _, method := range t.Methods.List {
										for _, name := range method.Names {
											// 获取方法对象及其签名
											methodObj := pkg.TypesInfo.Defs[name]
											if methodObj == nil {
												continue
											}

											methodFunc, ok := methodObj.(*types.Func)
											if !ok {
												continue
											}

											iface.Methods[name.Name] = methodFunc
										}
									}
								}

								// 排除空接口 (interface{})
								if len(iface.Methods) == 0 {
									continue
								}

								interfacesInfo[id] = iface

								// 初始化接口方法实现映射
								interfaceImpls.MethodImplementersMap[id] = make(map[string][]string)
								for methodName := range iface.Methods {
									interfaceImpls.MethodImplementersMap[id][methodName] = []string{}
								}
							}
						}
					}
				}
			}
		}
	}


	// key: 类型ID, value: 方法名 -> 节点ID
	typeMethodsMap := make(map[string]map[string]string)

	// 提取所有类型的方法
	for nodeID, node := range nodesInfo {
		// 检查是否为方法声明（形如 (Type).Method 的名称）
		if strings.HasPrefix(node.Name, "(") && strings.Contains(node.Name, ").") {
			// 解析接收器类型和方法名
			parts := strings.SplitN(node.Name, ").", 2)
			if len(parts) != 2 {
				continue
			}

			// 获取接收器类型名称（去掉括号和星号等）
			recvType := strings.TrimPrefix(parts[0], "(")
			recvType = strings.TrimPrefix(recvType, "*") // 处理指针接收器

			// 获取方法名
			methodName := parts[1]

			// 构造类型的完整ID
			typeFile := node.File
			pkgName := strings.TrimSuffix(nodeID, "/"+typeFile+":"+node.Name)
			typeID := GetObjectID(pkgName, typeFile, recvType) // 当前receiver的唯一标识

			if _, ok := typeMethodsMap[typeID]; !ok {
				typeMethodsMap[typeID] = make(map[string]string)
			}
			typeMethodsMap[typeID][methodName] = nodeID
		}
	}

	// 遍历所有顶层声明，组装接口表
	for nodeID, node := range nodesInfo {
		if node.Obj == nil {
			continue
		}

		// 只处理类型声明
		typeNameObj, ok := node.Obj.(*types.TypeName)
		if !ok {
			continue
		}

		typeObj := typeNameObj.Type()
		if typeObj == nil {
			continue
		}

		// 确保是命名类型
		if _, ok := typeObj.(*types.Named); !ok {
			continue
		}

		// 判断这个类型是否实现了任何接口
		for ifaceID, ifaceInfo := range interfacesInfo {
			// 跳过自身
			if ifaceID == nodeID {
				continue
			}

			// 记录实现了接口的哪些方法
			implemented := make(map[string]bool)

			// 检查这个类型是否实现了接口的所有方法
			allImplemented := true
			for methodName, ifaceMethod := range ifaceInfo.Methods {
				// 检查类型是否有这个方法
				methodFound := false

				// 获取接口方法签名
				ifaceMethodSig, ok := ifaceMethod.Type().(*types.Signature)
				if !ok {
					continue
				}

				// 查找类型的方法集合中是否包含此方法
				if methods, ok := typeMethodsMap[nodeID]; ok {
					if methodID, found := methods[methodName]; found {
						// 获取类型方法对象
						methodNode := nodesInfo[methodID]
						if methodNode == nil || methodNode.Obj == nil {
							continue
						}

						typeMethod, ok := methodNode.Obj.(*types.Func)
						if !ok {
							continue
						}

						// 获取类型方法签名
						typeMethodSig, ok := typeMethod.Type().(*types.Signature)
						if !ok {
							continue
						}

						// 检查方法签名是否匹配
						if signaturesCompatible(ifaceMethodSig, typeMethodSig) {
							methodFound = true
							// 记录该类型实现了接口的这个方法
							interfaceImpls.MethodImplementersMap[ifaceID][methodName] = append(
								interfaceImpls.MethodImplementersMap[ifaceID][methodName],
								methodID,
							)
							implemented[methodName] = true
						}
					}
				}

				if !methodFound {
					// 如果缺少任何方法或签名不匹配，则不完全实现接口
					allImplemented = false
					break
				}
			}

			// 如果实现了所有方法，则记录为接口的实现者
			if allImplemented && len(implemented) == len(ifaceInfo.Methods) {
				interfaceImpls.ImplementersMap[ifaceID] = append(interfaceImpls.ImplementersMap[ifaceID], nodeID)
			}
		}
	}
	// 辅助函数：处理一个AST节点（函数体或变量初始化表达式）来查找依赖的顶级对象
	collectDependencies := func(n ast.Node, curNodeID string, pkg *packages.Package) {
		ast.Inspect(n, func(n ast.Node) bool {
			// 处理选择器表达式（如 a.b 形式的调用）
			if sel, ok := n.(*ast.SelectorExpr); ok {
				var obj types.Object
				var objType types.Type

				// 处理嵌套选择器表达式 (如 a.b.c)
				switch x := sel.X.(type) {
				case *ast.Ident:
					// 简单情况: a.b
					obj = pkg.TypesInfo.Uses[x]
					if obj == nil {
						return true
					}
					objType = obj.Type()
				case *ast.SelectorExpr:
					// 嵌套情况: a.b.c
					// 获取 a.b 的类型信息
					exprType := pkg.TypesInfo.Types[x]
					if !exprType.IsValue() {
						return true
					}
					objType = exprType.Type
				default:
					// 其他复杂表达式，获取表达式类型
					exprType := pkg.TypesInfo.Types[sel.X]
					if !exprType.IsValue() {
						return true
					}
					objType = exprType.Type
				}

				if objType == nil {
					return true
				}

				// 查找右侧方法调用
				methodName := sel.Sel.Name
				// 检查对象类型是否是接口类型
				_, isIface := objType.Underlying().(*types.Interface)

				// 处理指针类型的情况
				if !isIface {
					if ptr, ok := objType.(*types.Pointer); ok {
						_, isIface = ptr.Elem().Underlying().(*types.Interface)
					}
				}

				if isIface {
					// 它是接口类型，需要查找所有实现了这个接口的类型
					// 首先尝试查找精确匹配的接口（如果已经记录在 interfacesInfo 中）
					foundExactMatch := false

					// 遍历已知的接口
					for ifaceID, ifaceInfo := range interfacesInfo {
						// 检查这个接口是否包含调用的方法
						if _, ok := ifaceInfo.Methods[methodName]; ok {
							// 找到接口对应的所有实现类型
							if impls, ok := interfaceImpls.ImplementersMap[ifaceID]; ok && len(impls) > 0 {
								foundExactMatch = true
								for _, implTypeID := range impls {
									// 查找实现类型的对应方法
									if typeMethods, ok := typeMethodsMap[implTypeID]; ok {
										if methodID, found := typeMethods[methodName]; found {
											// 添加从当前节点到实现类型方法的依赖关系
											graph[curNodeID][methodID] = struct{}{}
										}
									}
								}
							}
						}
					}

					// 如果没有找到精确匹配，则回退到查找所有包含该方法名的接口实现
					if !foundExactMatch {
						for _, methodImpls := range interfaceImpls.MethodImplementersMap {
							if impls, ok := methodImpls[methodName]; ok && len(impls) > 0 {
								for _, implID := range impls {
									graph[curNodeID][implID] = struct{}{}
								}
							}
						}
					}
				} else {
					// 检查是否有任何接口包含这个方法名
					for ifaceID, ifaceInfo := range interfacesInfo {
						// 检查该方法是否属于接口
						if _, ok := ifaceInfo.Methods[methodName]; ok {
							// 找到所有实现该接口方法的类型
							if impls, ok := interfaceImpls.MethodImplementersMap[ifaceID][methodName]; ok {
								for _, implID := range impls {
									// 添加依赖关系
									graph[curNodeID][implID] = struct{}{}
								}
							}
						}
					}
				}
				return true
			}

			ident, ok := n.(*ast.Ident)
			if !ok {
				return true
			}

			// 获取标识符对应的对象
			obj := pkg.TypesInfo.Uses[ident]
			if obj == nil {
				return true
			}
			// 判断这个对象是否在我们的顶级声明中（只考虑同一项目内部）
			if depID, ok := nodesMap[obj]; ok {
				// 避免自引用
				if depID != curNodeID {
					graph[curNodeID][depID] = struct{}{}
				}
			}
			return true
		})
	}

	// 第二次遍历，遍历各个顶级声明对应的初始化或函数体，建立依赖关系
	for _, pkg := range pkgs {
		for _, file := range pkg.Syntax {
			fullFilename := pkg.Fset.File(file.Pos()).Name()
			baseFilename := filepath.Base(fullFilename)
			for _, decl := range file.Decls {
				switch d := decl.(type) {
				case *ast.FuncDecl:
					if d.Name == nil || d.Body == nil {
						continue
					}
					funcName := GetFuncOrMethodName(d)
					curID := GetObjectID(pkg.ID, baseFilename, funcName)
					collectDependencies(d.Body, curID, pkg)
				case *ast.GenDecl:
					for _, spec := range d.Specs {
						switch s := spec.(type) {
						case *ast.ValueSpec:
							for _, ident := range s.Names {
								curID := GetObjectID(pkg.ID, baseFilename, ident.Name)
								// 如果有初始化表达式，则扫描之
								for _, expr := range s.Values {
									collectDependencies(expr, curID, pkg)
								}
							}
						}
					}
				}
			}
		}
	}

	// 输出接口实现的信息（用于调试）
	// fmt.Println("\n接口实现关系:")
	// for ifaceID, impls := range interfaceImpls.ImplementersMap {
	// 	if len(impls) > 0 {
	// 		fmt.Printf("接口 %s 的实现类型:\n", ifaceID)
	// 		for _, impl := range impls {
	// 			fmt.Printf("  - %s\n", impl)
	// 		}
	// 	}
	// }

	// 构建反向图
	revGraph := make(Graph)
	for nodeID, deps := range graph {
		for dep := range deps {
			if _, ok := revGraph[dep]; !ok {
				revGraph[dep] = make(map[string]struct{})
			}
			revGraph[dep][nodeID] = struct{}{}
		}
	}

	return &DependencyInfo{
		nodes:    nodesInfo,
		revGraph: revGraph,
	}, nil
}

// exprToString 返回表达式的字符串表示（对标识符和星号类型作简单处理）
func exprToString(expr ast.Expr) string {
	switch e := expr.(type) {
	case *ast.Ident:
		return e.Name
	case *ast.StarExpr:
		return "*" + exprToString(e.X)
	case *ast.IndexExpr:
		return exprToString(e.X) + "[" + exprToString(e.Index) + "]"
	case *ast.SelectorExpr:
		return exprToString(e.X) + "." + e.Sel.Name
	default:
		return fmt.Sprint(e)
	}
}

// GetFuncOrMethodName 获取函数或方法的名称
func GetFuncOrMethodName(fn *ast.FuncDecl) string {
	recv := ""
	if fn.Recv != nil && len(fn.Recv.List) > 0 {
		recv = exprToString(fn.Recv.List[0].Type)
	}
	if recv == "" {
		return fn.Name.Name
	}
	return fmt.Sprintf("(%s).%s", recv, fn.Name.Name)
}

// signaturesCompatible 检查类型方法的签名是否与接口方法的签名兼容
func signaturesCompatible(ifaceMethodSig, typeMethodSig *types.Signature) bool {
	// 检查参数数量是否相同
	if ifaceMethodSig.Params().Len() != typeMethodSig.Params().Len() {
		return false
	}

	// 检查返回值数量是否相同
	if ifaceMethodSig.Results().Len() != typeMethodSig.Results().Len() {
		return false
	}

	// 比较参数类型（忽略接收器）
	for i := 0; i < ifaceMethodSig.Params().Len(); i++ {
		ifaceParam := ifaceMethodSig.Params().At(i)
		typeParam := typeMethodSig.Params().At(i)

		if !types.AssignableTo(typeParam.Type(), ifaceParam.Type()) {
			return false
		}
	}

	// 比较返回值类型
	for i := 0; i < ifaceMethodSig.Results().Len(); i++ {
		ifaceResult := ifaceMethodSig.Results().At(i)
		typeResult := typeMethodSig.Results().At(i)

		if !types.AssignableTo(typeResult.Type(), ifaceResult.Type()) {
			return false
		}
	}

	// 检查可变参数特性是否一致
	if ifaceMethodSig.Variadic() != typeMethodSig.Variadic() {
		return false
	}

	return true
}
