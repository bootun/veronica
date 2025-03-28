package parser

import (
	"go/ast"
	"go/parser"
	"go/token"
	"testing"
)

func TestBuildDependency(t *testing.T) {
	pkgs, err := LoadPackages("./material")
	if err != nil {
		t.Fatalf("failed to load packages: %v", err)
	}

	depInfo, err := BuildDependency(pkgs)
	if err != nil {
		t.Fatalf("failed to build dependency: %v", err)
	}

	t.Logf("depInfo: %v", depInfo)
}

func TestGetFuncOrMethodName(t *testing.T) {
	type args struct {
		fn string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		// 函数
		{name: "func1", args: args{fn: "func Func1() {}"}, want: "Func1"},
		{name: "func2", args: args{fn: "func Func2() int { return 1 }"}, want: "Func2"},
		{name: "func3", args: args{fn: "func Func3() (int, error) { return 1, nil }"}, want: "Func3"},
		{name: "func4", args: args{fn: "func Func4(a int, b int) (int, error) { return a + b, nil }"}, want: "Func4"},
		{name: "func5", args: args{fn: "func Func5(a int, b int) (int, error) { return a + b, nil }"}, want: "Func5"},
		// 方法
		{name: "method1", args: args{fn: "func (m *MyType) Method1() { return 1 }"}, want: "(*MyType).Method1"},
		{name: "method2", args: args{fn: "func (m *MyType) Method2() (int, error) { return 1, nil }"}, want: "(*MyType).Method2"},
		// 范型
		{name: "genericFunc1", args: args{fn: "func GenericFunc[T any](a T) T { return a }"}, want: "GenericFunc"},
		{name: "genericFunc2", args: args{fn: "func GenericFunc2[T any, U any](a T, b U) (T, U) { return a, b }"}, want: "GenericFunc2"},
		{name: "genericFunc3", args: args{fn: "func GenericFunc3[T any](a T, b int) (T, int) { return a, b }"}, want: "GenericFunc3"},
		// 泛型方法
		{name: "genericMethod1", args: args{fn: "func (a *AutoFlushBuffer[T]) WriteMessage(msg T) { return a }"}, want: "(*AutoFlushBuffer[T]).WriteMessage"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			src := "package test\n\n" + tt.args.fn
			fset := token.NewFileSet()
			f, err := parser.ParseFile(fset, "test.go", src, parser.ParseComments)
			if err != nil {
				t.Fatalf("failed to parse file: %v", err)
			}

			// 获取第一个函数声明
			var funcDecl *ast.FuncDecl
			for _, decl := range f.Decls {
				if fd, ok := decl.(*ast.FuncDecl); ok {
					funcDecl = fd
					break
				}
			}

			if funcDecl == nil {
				t.Fatal("no function declaration found")
			}

			if got := GetFuncOrMethodName(funcDecl); got != tt.want {
				t.Errorf("GetFuncOrMethodName() = %s, want %v", got, tt.want)
			}
		})
	}
}
