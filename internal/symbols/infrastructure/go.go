package infrastructure

import (
	"context"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"harnessforge/internal/symbols/domain"
)

type Go struct{}

func (Go) Parse(ctx context.Context, path string, source []byte) ([]domain.Symbol, error) {
	set := token.NewFileSet()
	file, err := parser.ParseFile(set, path, source, 0)
	if err != nil {
		return nil, fmt.Errorf("parse Go source: %w", err)
	}
	var symbols []domain.Symbol
	interfaces := map[string]map[string]bool{}
	types := map[string]bool{}
	methods := map[string]map[string]bool{}
	for _, declaration := range file.Decls {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		switch value := declaration.(type) {
		case *ast.GenDecl:
			for _, specification := range value.Specs {
				typed, ok := specification.(*ast.TypeSpec)
				if !ok {
					continue
				}
				kind := "type"
				if interfaceType, ok := typed.Type.(*ast.InterfaceType); ok {
					kind = "interface"
					interfaces[typed.Name.Name] = interfaceMethods(interfaceType)
				} else {
					types[typed.Name.Name] = true
				}
				symbols = append(symbols, goSymbol(set, path, kind, typed.Name.Name, typed.Pos(), typed.End(), ""))
			}
		case *ast.FuncDecl:
			kind, receiver := "function", ""
			if value.Recv != nil && len(value.Recv.List) > 0 {
				kind, receiver = "method", goTypeName(value.Recv.List[0].Type)
				if methods[receiver] == nil {
					methods[receiver] = map[string]bool{}
				}
				methods[receiver][value.Name.Name] = true
			}
			symbols = append(symbols, goSymbol(set, path, kind, value.Name.Name, value.Pos(), value.End(), receiver))
		}
	}
	for i := range symbols {
		if !types[symbols[i].Name] {
			continue
		}
		for name, required := range interfaces {
			if includesMethods(methods[symbols[i].Name], required) {
				symbols[i].Kind = "implementation"
				symbols[i].Implements = append(symbols[i].Implements, name)
			}
		}
	}
	return symbols, nil
}

func interfaceMethods(value *ast.InterfaceType) map[string]bool {
	methods := map[string]bool{}
	for _, field := range value.Methods.List {
		for _, name := range field.Names {
			methods[name.Name] = true
		}
	}
	return methods
}

func includesMethods(actual, required map[string]bool) bool {
	if len(required) == 0 {
		return false
	}
	for name := range required {
		if !actual[name] {
			return false
		}
	}
	return true
}

func goSymbol(set *token.FileSet, path, kind, name string, start, end token.Pos, receiver string) domain.Symbol {
	return domain.Symbol{Kind: kind, Name: name, File: path, StartLine: set.Position(start).Line, EndLine: set.Position(end).Line, Receiver: receiver}
}

func goTypeName(expression ast.Expr) string {
	switch value := expression.(type) {
	case *ast.Ident:
		return value.Name
	case *ast.StarExpr:
		return goTypeName(value.X)
	}
	return ""
}
