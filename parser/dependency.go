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

// interfaceInfo store interface related information
type interfaceInfo struct {
	// key: method name, value: method signature
	Methods map[string]*methodInfo
	// all node ids that implements this interface
	Implements []string
}

type methodInfo struct {
	Method *types.Func
	// all node ids that implements this method
	ImplementMethods []string
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

func GetNodeId(pkg *packages.Package, node ast.Node) string {
	fset := pkg.Fset
	fullFilename := fset.File(node.Pos()).Name()
	baseFilename := filepath.Base(fullFilename)
	switch n := node.(type) {
	case *ast.FuncDecl:
		funcName := GetFuncOrMethodName(n)
		return GetObjectID(pkg.ID, baseFilename, funcName)
	case *ast.ValueSpec:
		return GetObjectID(pkg.ID, baseFilename, n.Names[0].Name)
	case *ast.TypeSpec:
		return GetObjectID(pkg.ID, baseFilename, n.Name.Name)
	default:
		panic(fmt.Sprintf("unsupported node type: %T", n))
	}
	return ""
}

type DependencyInfo struct {
	// 项目内所有的顶级声明, key: NodeID, value: Node
	nodes map[string]*node
	// 依赖图的反向图, key: NodeID, value: 依赖key的NodeID列表
	revGraph Graph
}

// GetDependency 获取 targetID 的依赖节点
func (d *DependencyInfo) GetDependency(targetID string) ([]string, error) {
	if _, ok := d.nodes[targetID]; !ok {
		return nil, fmt.Errorf("target %s is not defined in project", targetID)
	}

	visited := make(map[string]struct{})
	var dfs func(string)
	dfs = func(node string) {
		for dep := range d.revGraph[node] {
			if _, ok := visited[dep]; !ok {
				visited[dep] = struct{}{}
				dfs(dep)
			}
		}
	}
	dfs(targetID)

	deps := make([]string, 0, len(visited))
	for id := range visited {
		deps = append(deps, id)
	}
	return deps, nil
}

// BuildDependency 构建依赖关系图
func BuildDependency(pkgs []*packages.Package) (*DependencyInfo, error) {
	// nodesMap：key: 对象, value: 节点唯一标识
	// 项目内所有的顶级声明
	nodesMap := make(map[types.Object]string)
	// nodesInfo：存储每个节点的信息
	// key: NodeID, value: Node
	nodesInfo := make(map[string]*node)

	// key: InterfaceID, value: 接口信息
	interfaceMap := make(map[string]*interfaceInfo)

	// 依赖图: key->NodeID, value->依赖Key的NodeID列表
	graph := make(Graph)

	// 遍历所有包和文件，提取顶级声明，构建接口表
	for _, pkg := range pkgs {
		fset := pkg.Fset
		for _, file := range pkg.Syntax {
			fullFilename := fset.File(file.Pos()).Name()
			baseFilename := filepath.Base(fullFilename)
			// 遍历文件中的所有顶级声明
			for _, decl := range file.Decls {
				switch d := decl.(type) {
				// function or method
				case *ast.FuncDecl:
					if d.Name == nil {
						continue
					}
					funcName := GetFuncOrMethodName(d)
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

				// constant, type or variable declaration
				case *ast.GenDecl:
					for _, spec := range d.Specs {
						switch s := spec.(type) {
						// constant or variable declaration
						case *ast.ValueSpec:
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
							}

						// type declaration
						case *ast.TypeSpec:
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

							// handle type parameters(generic type)
							typeParams := make(map[string]*types.TypeParam)
							if s.TypeParams != nil && len(s.TypeParams.List) > 0 {
								for _, field := range s.TypeParams.List {
									for _, name := range field.Names {
										// 获取类型参数对象
										typeParamObj := pkg.TypesInfo.Defs[name]
										if typeParamObj == nil {
											continue
										}

										// 转换为TypeParam类型
										typeParam, ok := typeParamObj.Type().(*types.TypeParam)
										if !ok {
											continue
										}

										// 存储类型参数信息
										typeParams[name.Name] = typeParam

										// 处理类型参数之间的依赖关系
										if field.Type != nil {
											// 解析类型参数的约束，添加对其他类型的依赖
											ast.Inspect(field.Type, func(n ast.Node) bool {
												if ident, ok := n.(*ast.Ident); ok && ident.Name != name.Name {
													// 获取约束中引用的类型
													if refObj := pkg.TypesInfo.Uses[ident]; refObj != nil {
														if depID, exists := nodesMap[refObj]; exists && depID != id {
															addDependency(graph, id, depID)
														}
													}
												}
												return true
											})
										}
									}
								}
							}

							// 检查是否为接口定义
							if t, ok := s.Type.(*ast.InterfaceType); ok {
								iface := parseAstInterfaceType(pkg, t)
								if iface == nil {
									continue
								}
								interfaceMap[id] = iface

							} else {
								// 非接口类型定义，处理泛型类型参数在类型定义中的使用
								ast.Inspect(s.Type, func(n ast.Node) bool {
									if ident, ok := n.(*ast.Ident); ok {
										// 检查是否引用了其他类型
										if refObj := pkg.TypesInfo.Uses[ident]; refObj != nil {
											if depID, exists := nodesMap[refObj]; exists && depID != id {
												addDependency(graph, id, depID)
											}
										}
									}
									return true
								})
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

	err := parseInterfaceImplementations(nodesInfo, interfaceMap, typeMethodsMap)
	if err != nil {
		return nil, fmt.Errorf("failed to parse interface implementations: %w", err)
	}
	// 辅助函数：处理一个AST节点（函数体或变量初始化表达式）来查找依赖的顶级对象
	collectDependencies := func(n ast.Node, curNodeID string, pkg *packages.Package) {
		ast.Inspect(n, func(n ast.Node) bool {
			// 处理类型转换和类型断言
			if typeAssert, ok := n.(*ast.TypeAssertExpr); ok {
				if typeAssert.Type != nil {
					ast.Inspect(typeAssert.Type, func(n ast.Node) bool {
						if ident, ok := n.(*ast.Ident); ok {
							if obj := pkg.TypesInfo.Uses[ident]; obj != nil {
								if depID, ok := nodesMap[obj]; ok && depID != curNodeID {
									addDependency(graph, curNodeID, depID)
								}
							}
						}
						return true
					})
				}
			}

			// 处理结构体字面量
			if compLit, ok := n.(*ast.CompositeLit); ok {
				if compLit.Type != nil {
					ast.Inspect(compLit.Type, func(n ast.Node) bool {
						if ident, ok := n.(*ast.Ident); ok {
							if obj := pkg.TypesInfo.Uses[ident]; obj != nil {
								if depID, ok := nodesMap[obj]; ok && depID != curNodeID {
									addDependency(graph, curNodeID, depID)
								}
							}
						}
						return true
					})
				}
				// 处理结构体字段
				for _, elt := range compLit.Elts {
					if kv, ok := elt.(*ast.KeyValueExpr); ok {
						if ident, ok := kv.Key.(*ast.Ident); ok {
							if obj := pkg.TypesInfo.Uses[ident]; obj != nil {
								if depID, ok := nodesMap[obj]; ok && depID != curNodeID {
									addDependency(graph, curNodeID, depID)
								}
							}
						}
					}
				}
			}

			// 处理函数类型（包括参数和返回值）
			if funcType, ok := n.(*ast.FuncType); ok {
				// 处理参数
				if funcType.Params != nil {
					for _, field := range funcType.Params.List {
						ast.Inspect(field.Type, func(n ast.Node) bool {
							if ident, ok := n.(*ast.Ident); ok {
								if obj := pkg.TypesInfo.Uses[ident]; obj != nil {
									if depID, ok := nodesMap[obj]; ok && depID != curNodeID {
										addDependency(graph, curNodeID, depID)
									}
								}
							}
							return true
						})
					}
				}
				// 处理返回值
				if funcType.Results != nil {
					for _, field := range funcType.Results.List {
						ast.Inspect(field.Type, func(n ast.Node) bool {
							if ident, ok := n.(*ast.Ident); ok {
								if obj := pkg.TypesInfo.Uses[ident]; obj != nil {
									if depID, ok := nodesMap[obj]; ok && depID != curNodeID {
										addDependency(graph, curNodeID, depID)
									}
								}
							}
							return true
						})
					}
				}
			}

			// 处理标签
			if label, ok := n.(*ast.LabeledStmt); ok {
				if obj := pkg.TypesInfo.Defs[label.Label]; obj != nil {
					if depID, ok := nodesMap[obj]; ok && depID != curNodeID {
						addDependency(graph, curNodeID, depID)
					}
				}
			}

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

				// 检查是否是泛型方法调用
				if methodType, ok := pkg.TypesInfo.Types[sel].Type.(*types.Signature); ok {
					// 处理泛型方法的类型参数
					if methodType.TypeParams() != nil && methodType.TypeParams().Len() > 0 {
						// 处理泛型方法的类型参数
						for i := 0; i < methodType.TypeParams().Len(); i++ {
							typeParam := methodType.TypeParams().At(i)
							if depID, ok := nodesMap[typeParam.Obj()]; ok && depID != curNodeID {
								addDependency(graph, curNodeID, depID)
							}
						}
					}

					// 处理泛型方法的约束
					if methodType.TypeParams() != nil && methodType.TypeParams().Len() > 0 {
						for i := 0; i < methodType.TypeParams().Len(); i++ {
							typeParam := methodType.TypeParams().At(i)
							// 获取类型参数的约束
							if constraint := typeParam.Constraint(); constraint != nil {
								// 处理约束中的类型
								ast.Inspect(sel, func(n ast.Node) bool {
									if ident, ok := n.(*ast.Ident); ok {
										if obj := pkg.TypesInfo.Uses[ident]; obj != nil {
											if depID, ok := nodesMap[obj]; ok && depID != curNodeID {
												addDependency(graph, curNodeID, depID)
											}
										}
									}
									return true
								})
							}
						}
					}

					// 处理泛型方法的接收器
					if recv := methodType.Recv(); recv != nil {
						recvType := recv.Type()
						// 处理指针接收器
						if ptr, ok := recvType.(*types.Pointer); ok {
							recvType = ptr.Elem()
						}
						// 处理命名类型
						if named, ok := recvType.(*types.Named); ok {
							// 处理接收器类型的类型参数
							if named.TypeParams() != nil && named.TypeParams().Len() > 0 {
								for i := 0; i < named.TypeParams().Len(); i++ {
									typeParam := named.TypeParams().At(i)
									if depID, ok := nodesMap[typeParam.Obj()]; ok && depID != curNodeID {
										addDependency(graph, curNodeID, depID)
									}
								}
							}
							// 处理接收器类型本身
							if depID, ok := nodesMap[named.Obj()]; ok && depID != curNodeID {
								addDependency(graph, curNodeID, depID)
							}
						}
					}
				}

				// 处理泛型方法的调用参数
				if call, ok := sel.X.(*ast.CallExpr); ok {
					// 处理调用参数中的类型
					for _, arg := range call.Args {
						ast.Inspect(arg, func(n ast.Node) bool {
							if ident, ok := n.(*ast.Ident); ok {
								if obj := pkg.TypesInfo.Uses[ident]; obj != nil {
									if depID, ok := nodesMap[obj]; ok && depID != curNodeID {
										addDependency(graph, curNodeID, depID)
									}
								}
							}
							return true
						})
					}
				}

				// 处理泛型方法的类型参数使用
				if methodType, ok := pkg.TypesInfo.Types[sel].Type.(*types.Signature); ok {
					// 处理参数中的类型参数使用
					if methodType.Params() != nil {
						for i := 0; i < methodType.Params().Len(); i++ {
							param := methodType.Params().At(i)
							paramType := param.Type()
							// 检查参数类型是否是类型参数
							if typeParam, ok := paramType.(*types.TypeParam); ok {
								if depID, ok := nodesMap[typeParam.Obj()]; ok && depID != curNodeID {
									addDependency(graph, curNodeID, depID)
								}
							}
							// 检查参数类型是否包含类型参数
							if named, ok := paramType.(*types.Named); ok {
								if named.TypeParams() != nil && named.TypeParams().Len() > 0 {
									for j := 0; j < named.TypeParams().Len(); j++ {
										typeParam := named.TypeParams().At(j)
										if depID, ok := nodesMap[typeParam.Obj()]; ok && depID != curNodeID {
											addDependency(graph, curNodeID, depID)
										}
									}
								}
							}
						}
					}
					// 处理返回值中的类型参数使用
					if methodType.Results() != nil {
						for i := 0; i < methodType.Results().Len(); i++ {
							result := methodType.Results().At(i)
							resultType := result.Type()
							// 检查返回值类型是否是类型参数
							if typeParam, ok := resultType.(*types.TypeParam); ok {
								if depID, ok := nodesMap[typeParam.Obj()]; ok && depID != curNodeID {
									addDependency(graph, curNodeID, depID)
								}
							}
							// 检查返回值类型是否包含类型参数
							if named, ok := resultType.(*types.Named); ok {
								if named.TypeParams() != nil && named.TypeParams().Len() > 0 {
									for j := 0; j < named.TypeParams().Len(); j++ {
										typeParam := named.TypeParams().At(j)
										if depID, ok := nodesMap[typeParam.Obj()]; ok && depID != curNodeID {
											addDependency(graph, curNodeID, depID)
										}
									}
								}
							}
						}
					}
				}

				if isIface {
					// 它是接口类型，需要查找所有实现了这个接口的类型
					// 首先尝试查找精确匹配的接口（如果已经记录在 interfacesInfo 中）
					foundExactMatch := false

					// 遍历已知的接口
					for _, ifaceInfo := range interfaceMap {
						// 检查这个接口是否包含调用的方法
						if _, ok := ifaceInfo.Methods[methodName]; ok {
							// 找到接口对应的所有实现类型
							for _, implTypeID := range ifaceInfo.Implements {
								foundExactMatch = true
								// 查找实现类型的对应方法
								if typeMethods, ok := typeMethodsMap[implTypeID]; ok {
									if methodID, found := typeMethods[methodName]; found {
										// 添加从当前节点到实现类型方法的依赖关系
										addDependency(graph, curNodeID, methodID)
									}
								}
							}
						}
					}

					// 如果没有找到精确匹配，则回退到查找所有包含该方法名的接口实现
					if !foundExactMatch {
						for _, ifaceInfo := range interfaceMap {
							if _, ok := ifaceInfo.Methods[methodName]; ok {
								for _, implID := range ifaceInfo.Methods[methodName].ImplementMethods {
									addDependency(graph, curNodeID, implID)
								}
							}
						}
					}
				} else {
					// 检查是否有任何接口包含这个方法名
					for _, ifaceInfo := range interfaceMap {
						// 检查该方法是否属于接口
						if _, ok := ifaceInfo.Methods[methodName]; ok {
							// 找到所有实现该接口方法的类型
							for _, implID := range ifaceInfo.Methods[methodName].ImplementMethods {
								addDependency(graph, curNodeID, implID)
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
			// 如果obj是函数，则获取其原始函数
			if f, ok := obj.(*types.Func); ok {
				if f.Origin() != nil {
					obj = f.Origin()
				}
			}
			// 判断这个对象是否在我们的顶级声明中（只考虑同一项目内部）
			if depID, ok := nodesMap[obj]; ok {
				// 避免自引用
				if depID != curNodeID {
					addDependency(graph, curNodeID, depID)
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

					// 处理函数参数
					if d.Type.Params != nil {
						for _, field := range d.Type.Params.List {
							ast.Inspect(field.Type, func(n ast.Node) bool {
								if ident, ok := n.(*ast.Ident); ok {
									if obj := pkg.TypesInfo.Uses[ident]; obj != nil {
										if depID, ok := nodesMap[obj]; ok && depID != curID {
											addDependency(graph, curID, depID)
										}
									}
								}
								return true
							})
						}
					}

					// 处理函数返回值
					if d.Type.Results != nil {
						for _, field := range d.Type.Results.List {
							ast.Inspect(field.Type, func(n ast.Node) bool {
								if ident, ok := n.(*ast.Ident); ok {
									if obj := pkg.TypesInfo.Uses[ident]; obj != nil {
										if depID, ok := nodesMap[obj]; ok && depID != curID {
											addDependency(graph, curID, depID)
										}
									}
								}
								return true
							})
						}
					}

					// 处理函数接收器
					if d.Recv != nil && len(d.Recv.List) > 0 {
						for _, field := range d.Recv.List {
							ast.Inspect(field.Type, func(n ast.Node) bool {
								if ident, ok := n.(*ast.Ident); ok {
									if obj := pkg.TypesInfo.Uses[ident]; obj != nil {
										if depID, ok := nodesMap[obj]; ok && depID != curID {
											addDependency(graph, curID, depID)
										}
									}
								}
								return true
							})
						}
					}
					// 处理函数体
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

// parseAstInterfaceType 初始化AST接口类型
func parseAstInterfaceType(pkg *packages.Package, t *ast.InterfaceType) *interfaceInfo {
	// 记录接口信息
	iface := &interfaceInfo{
		Methods:    make(map[string]*methodInfo),
		Implements: make([]string, 0),
	}

	if t.Methods != nil {
		for _, method := range t.Methods.List {
			// in interface, method.Names length always is 1
			if len(method.Names) != 1 {
				panic(fmt.Sprintf("invalid interface method: %v", GetNodeId(pkg, method)))
			}
			ident := method.Names[0]
			methodObj := pkg.TypesInfo.Defs[ident]
			if methodObj == nil {
				continue
			}
			methodFunc, ok := methodObj.(*types.Func)
			if !ok {
				continue
			}

			iface.Methods[ident.Name] = &methodInfo{
				Method: methodFunc,
			}
		}
	}

	// 排除空接口 (interface{})
	if len(iface.Methods) == 0 {
		return nil
	}

	return iface
}

// parseInterfaceImplementations 解析接口实现
func parseInterfaceImplementations(nodesInfo map[string]*node, interfacesInfo map[string]*interfaceInfo, typeMethodsMap map[string]map[string]string) error {
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
				ifaceMethodSig, ok := ifaceMethod.Method.Type().(*types.Signature)
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
							ifaceInfo.Methods[methodName].ImplementMethods = append(
								ifaceInfo.Methods[methodName].ImplementMethods,
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
				ifaceInfo.Implements = append(ifaceInfo.Implements, nodeID)
			}
		}
	}

	return nil
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
	// 检查接口方法是否有类型参数
	ifaceHasTypeParams := ifaceMethodSig.TypeParams() != nil && ifaceMethodSig.TypeParams().Len() > 0

	// 检查接收器是否有类型参数
	ifaceHasRecvTypeParams := false
	if recv := ifaceMethodSig.Recv(); recv != nil {
		// 获取接收器类型
		recvType := recv.Type()

		// 检查是否是命名类型
		if named, ok := recvType.(*types.Named); ok {
			// 检查命名类型是否有类型参数
			if named.TypeParams() != nil && named.TypeParams().Len() > 0 {
				ifaceHasRecvTypeParams = true
			}
		}
	}

	// 如果接口方法有类型参数或接收器有类型参数，标记为泛型方法
	hasGenericParams := ifaceHasTypeParams || ifaceHasRecvTypeParams

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

		ifaceParamType := ifaceParam.Type()
		typeParamType := typeParam.Type()

		// 如果接口有类型参数
		if hasGenericParams {
			// 检查参数本身是否是类型参数
			_, isTypeParam := ifaceParamType.(*types.TypeParam)
			if isTypeParam {
				// 泛型类型参数可以匹配任何类型，跳过详细检查
				continue
			}

			// 检查参数是否是包含泛型的复合类型

			// 处理切片类型 []T
			ifaceSlice, ifaceIsSlice := ifaceParamType.(*types.Slice)
			_, typeIsSlice := typeParamType.(*types.Slice)
			if ifaceIsSlice && typeIsSlice {
				// 获取切片元素类型
				ifaceElem := ifaceSlice.Elem()

				// 检查元素是否是类型参数
				_, elemIsTypeParam := ifaceElem.(*types.TypeParam)
				if elemIsTypeParam {
					// 如果接口参数是[]T，T是类型参数，则认为兼容任何切片类型
					continue
				}

				// 检查是否是命名类型且包含类型参数
				if named, ok := ifaceElem.(*types.Named); ok {
					if named.TypeParams() != nil && named.TypeParams().Len() > 0 {
						// 如果元素是泛型类型，也认为兼容
						continue
					}
				}
			}

			// 处理map类型 map[K]V
			ifaceMap, ifaceIsMap := ifaceParamType.(*types.Map)
			_, typeIsMap := typeParamType.(*types.Map)
			if ifaceIsMap && typeIsMap {
				// 获取map的键和值类型
				ifaceKey := ifaceMap.Key()
				ifaceVal := ifaceMap.Elem()

				// 检查键或值是否是类型参数
				_, keyIsTypeParam := ifaceKey.(*types.TypeParam)
				_, valIsTypeParam := ifaceVal.(*types.TypeParam)

				// 检查键和值是否是命名类型且包含类型参数
				keyHasGenericParams := false
				valHasGenericParams := false

				if named, ok := ifaceKey.(*types.Named); ok {
					if named.TypeParams() != nil && named.TypeParams().Len() > 0 {
						keyHasGenericParams = true
					}
				}

				if named, ok := ifaceVal.(*types.Named); ok {
					if named.TypeParams() != nil && named.TypeParams().Len() > 0 {
						valHasGenericParams = true
					}
				}

				// 如果键或值是类型参数或泛型类型，则认为兼容
				if keyIsTypeParam || valIsTypeParam || keyHasGenericParams || valHasGenericParams {
					continue
				}
			}

			// 处理通道类型 chan T
			ifaceChan, ifaceIsChan := ifaceParamType.(*types.Chan)
			typeChan, typeIsChan := typeParamType.(*types.Chan)
			if ifaceIsChan && typeIsChan {
				// 获取通道元素类型
				ifaceElem := ifaceChan.Elem()

				// 检查元素是否是类型参数
				_, elemIsTypeParam := ifaceElem.(*types.TypeParam)

				// 检查是否是命名类型且包含类型参数
				elemHasGenericParams := false
				if named, ok := ifaceElem.(*types.Named); ok {
					if named.TypeParams() != nil && named.TypeParams().Len() > 0 {
						elemHasGenericParams = true
					}
				}

				// 如果元素是类型参数或泛型类型，并且通道方向相同，则认为兼容
				if (elemIsTypeParam || elemHasGenericParams) && (ifaceChan.Dir() == typeChan.Dir()) {
					continue
				}
			}

			// 处理指针类型 *T
			ifacePtr, ifaceIsPtr := ifaceParamType.(*types.Pointer)
			_, typeIsPtr := typeParamType.(*types.Pointer)
			if ifaceIsPtr && typeIsPtr {
				// 获取指针的基础类型
				ifaceElem := ifacePtr.Elem()

				// 检查基础类型是否是类型参数
				_, elemIsTypeParam := ifaceElem.(*types.TypeParam)

				// 检查是否是命名类型且包含类型参数
				elemHasGenericParams := false
				if named, ok := ifaceElem.(*types.Named); ok {
					if named.TypeParams() != nil && named.TypeParams().Len() > 0 {
						elemHasGenericParams = true
					}
				}

				// 如果基础类型是类型参数或泛型类型，则认为兼容
				if elemIsTypeParam || elemHasGenericParams {
					continue
				}
			}

			// 检查命名类型是否包含类型参数
			if named, ok := ifaceParamType.(*types.Named); ok {
				if named.TypeParams() != nil && named.TypeParams().Len() > 0 {
					// 对于带有类型参数的命名类型，我们简化处理，认为兼容
					continue
				}
			}
		}

		// 对于非泛型参数或未能识别的泛型场景，使用标准的类型赋值检查
		if !types.AssignableTo(typeParamType, ifaceParamType) {
			if hasGenericParams {
				//panic(fmt.Sprintf("类型不兼容: %v 不能赋值给 %v", typeParamType, ifaceParamType))
				return false
			}
			return false
		}
	}

	// 比较返回值类型
	for i := 0; i < ifaceMethodSig.Results().Len(); i++ {
		ifaceResult := ifaceMethodSig.Results().At(i)
		typeResult := typeMethodSig.Results().At(i)

		ifaceResultType := ifaceResult.Type()
		typeResultType := typeResult.Type()

		// 如果接口有类型参数
		if hasGenericParams {
			// 检查返回值是否是类型参数
			_, isTypeParam := ifaceResultType.(*types.TypeParam)
			if isTypeParam {
				// 泛型类型参数可以匹配任何类型
				continue
			}

			// 检查返回值是否是包含泛型的复合类型

			// 处理切片类型 []T
			ifaceSlice, ifaceIsSlice := ifaceResultType.(*types.Slice)
			_, typeIsSlice := typeResultType.(*types.Slice)
			if ifaceIsSlice && typeIsSlice {
				// 获取切片元素类型
				ifaceElem := ifaceSlice.Elem()

				// 检查元素是否是类型参数
				_, elemIsTypeParam := ifaceElem.(*types.TypeParam)

				// 检查是否是命名类型且包含类型参数
				elemHasGenericParams := false
				if named, ok := ifaceElem.(*types.Named); ok {
					if named.TypeParams() != nil && named.TypeParams().Len() > 0 {
						elemHasGenericParams = true
					}
				}

				// 如果元素是类型参数或泛型类型，则认为兼容
				if elemIsTypeParam || elemHasGenericParams {
					continue
				}
			}

			// 处理map类型 map[K]V
			ifaceMap, ifaceIsMap := ifaceResultType.(*types.Map)
			_, typeIsMap := typeResultType.(*types.Map)
			if ifaceIsMap && typeIsMap {
				// 获取map的键和值类型
				ifaceKey := ifaceMap.Key()
				ifaceVal := ifaceMap.Elem()

				// 检查键或值是否是类型参数
				_, keyIsTypeParam := ifaceKey.(*types.TypeParam)
				_, valIsTypeParam := ifaceVal.(*types.TypeParam)

				// 检查键和值是否是命名类型且包含类型参数
				keyHasGenericParams := false
				valHasGenericParams := false

				if named, ok := ifaceKey.(*types.Named); ok {
					if named.TypeParams() != nil && named.TypeParams().Len() > 0 {
						keyHasGenericParams = true
					}
				}

				if named, ok := ifaceVal.(*types.Named); ok {
					if named.TypeParams() != nil && named.TypeParams().Len() > 0 {
						valHasGenericParams = true
					}
				}

				// 如果键或值是类型参数或泛型类型，则认为兼容
				if keyIsTypeParam || valIsTypeParam || keyHasGenericParams || valHasGenericParams {
					continue
				}
			}

			// 处理通道类型 chan T
			ifaceChan, ifaceIsChan := ifaceResultType.(*types.Chan)
			typeChan, typeIsChan := typeResultType.(*types.Chan)
			if ifaceIsChan && typeIsChan {
				// 获取通道元素类型
				ifaceElem := ifaceChan.Elem()

				// 检查元素是否是类型参数
				_, elemIsTypeParam := ifaceElem.(*types.TypeParam)

				// 检查是否是命名类型且包含类型参数
				elemHasGenericParams := false
				if named, ok := ifaceElem.(*types.Named); ok {
					if named.TypeParams() != nil && named.TypeParams().Len() > 0 {
						elemHasGenericParams = true
					}
				}

				// 如果元素是类型参数或泛型类型，并且通道方向相同，则认为兼容
				if (elemIsTypeParam || elemHasGenericParams) && (ifaceChan.Dir() == typeChan.Dir()) {
					continue
				}
			}

			// 处理指针类型 *T
			ifacePtr, ifaceIsPtr := ifaceResultType.(*types.Pointer)
			_, typeIsPtr := typeResultType.(*types.Pointer)
			if ifaceIsPtr && typeIsPtr {
				// 获取指针的基础类型
				ifaceElem := ifacePtr.Elem()

				// 检查基础类型是否是类型参数
				_, elemIsTypeParam := ifaceElem.(*types.TypeParam)

				// 检查是否是命名类型且包含类型参数
				elemHasGenericParams := false
				if named, ok := ifaceElem.(*types.Named); ok {
					if named.TypeParams() != nil && named.TypeParams().Len() > 0 {
						elemHasGenericParams = true
					}
				}

				// 如果基础类型是类型参数或泛型类型，则认为兼容
				if elemIsTypeParam || elemHasGenericParams {
					continue
				}
			}

			// 检查命名类型是否包含类型参数
			if named, ok := ifaceResultType.(*types.Named); ok {
				if named.TypeParams() != nil && named.TypeParams().Len() > 0 {
					// 对于带有类型参数的命名类型，我们简化处理，认为兼容
					continue
				}
			}
		}

		// 对于非泛型返回值或未能识别的泛型场景，使用标准的类型赋值检查
		if !types.AssignableTo(typeResultType, ifaceResultType) {
			return false
		}
	}

	// 检查可变参数特性是否一致
	return ifaceMethodSig.Variadic() == typeMethodSig.Variadic()
}

func LoadPackages(repo string) ([]*packages.Package, error) {
	cfg := &packages.Config{
		Mode: packages.NeedName | packages.NeedFiles | packages.NeedSyntax | packages.NeedTypes | packages.NeedTypesInfo,
		Dir:  repo,
	}
	pkgs, err := packages.Load(cfg, "./...")
	if err != nil {
		return nil, fmt.Errorf("parser project AST failed in %s: %v", repo, err)
	}
	return pkgs, nil
}

// addDependency adds a dependency edge from fromID to toID in the graph, avoiding self-references.
func addDependency(graph Graph, fromID, toID string) {
	if fromID == toID {
		return // Avoid self-references
	}
	if _, ok := graph[fromID]; !ok {
		graph[fromID] = make(map[string]struct{})
	}
	graph[fromID][toID] = struct{}{}
}
