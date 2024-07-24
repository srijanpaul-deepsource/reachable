package sniper

import (
	"context"
	"fmt"

	sitter "github.com/smacker/go-tree-sitter"
	treeSitterPy "github.com/smacker/go-tree-sitter/python"
)

type Python struct {
	module *Module
}

func (py *Python) Module() *Module {
	return py.module
}

func ParsePython(fileName string, source string) (*Python, error) {
	sourceBytes := []byte(source)
	python := &Python{module: &Module{
		FileName:   fileName,
		Source:     sourceBytes,
		TsLanguage: treeSitterPy.GetLanguage(),
	}}

	ast, err := sitter.ParseCtx(
		context.Background(), sourceBytes, python.module.TsLanguage,
	)

	if err != nil {
		return nil, err
	}
	python.module.Ast = ast

	scope, scopeMap := makeLexicalScopeTree(python, ast)
	python.module.GlobalScope = scope
	python.module.ScopeOfNode = scopeMap

	return python, nil
}

func (py *Python) GetDecls(node *sitter.Node) []Decl {
	switch node.Type() {
	case "assignment":
		{
			lhs := node.ChildByFieldName("left")
			rhs := node.ChildByFieldName("right")

			if lhs == nil || rhs == nil {
				return nil
			}

			if lhs.Type() == "identifier" {
				return []Decl{{lhs.Content(py.module.Source), rhs}}
			} else if lhs.Type() == "pattern_list" && rhs.Type() == "expression_list" {
				decls := []Decl{}

				nLeftChildren := int(lhs.NamedChildCount())
				nRightChildren := int(rhs.NamedChildCount())

				for i := 0; i < min(nLeftChildren, nRightChildren); i++ {
					if lhs.NamedChild(i).Type() == "identifier" {
						decls = append(decls, Decl{
							lhs.NamedChild(i).Content(py.module.Source),
							rhs.NamedChild(i),
						})
					}
					// TODO(@Srijan/Tushar): nested patterns and all that
				}

				return decls
			} else {
				panic(fmt.Sprintf("lhs type: %s, rhs: %s", lhs.Type(), rhs.Type()))
			}
			// TODO@(Srijan/Tushar) – Handle other assignment patterns
		}

	case "function_definition":
		{
			funcName := node.ChildByFieldName("name")
			if funcName != nil {
				return []Decl{{funcName.Content(py.module.Source), node}}
			}
			// TODO@(Srijan/Tushar) bind function parameters
		}
	}

	return nil
}

func (py *Python) IsCallExpr(node *sitter.Node) bool {
	return node.Type() == "call"
}

func (py *Python) IsFunctionDef(node *sitter.Node) bool {
	return node.Type() == "function_definition" || node.Type() == "lambda"
}

func (py *Python) GetCalleeName(node *sitter.Node) *string {
	function := node.ChildByFieldName("function")
	if function == nil {
		return nil
	}

	if function.Type() == "identifier" {
		name := function.Content(py.module.Source)
		return &name
	}

	return nil
}

func (py *Python) BodyOfFunction(node *sitter.Node) *sitter.Node {
	typ := node.Type()
	if typ != "function_definition" && typ != "lambda" {
		return nil
	}

	return node.ChildByFieldName("body")
}

func (py *Python) NameOfFunction(node *sitter.Node) string {
	if node.Type() == "function_definition" {
		return node.ChildByFieldName("name").Content(py.module.Source)
	}

	if node.Type() == "lambda" {
		nearestScope := GetScope(py.Module(), node)
		return nearestScope.NameOfNode[node]
	}

	return ""
}

func (py *Python) IsImport(node *sitter.Node) bool {
	return false
}

func (py *Python) FilePathOfImport(node *sitter.Node) string {
	cached := py.module.FilePathOfImport[node]
	if cached != "" {
		return cached
	}

	return ""
}
