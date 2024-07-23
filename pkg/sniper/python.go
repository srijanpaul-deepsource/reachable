package sniper

import (
	"context"
	"fmt"

	sitter "github.com/smacker/go-tree-sitter"
	treeSitterPy "github.com/smacker/go-tree-sitter/python"
)

type Python struct {
	ast         *sitter.Node
	scope       *Scope
	source      []byte
	scopeOfNode ScopeOfNode
}

func (p *Python) GetTreeSitterLanguage() *sitter.Language {
	return treeSitterPy.GetLanguage()
}

func ParsePython(source string) (Language, error) {
	sourceBytes := []byte(source)
	ast, err := sitter.ParseCtx(context.TODO(), sourceBytes, treeSitterPy.GetLanguage())
	if err != nil {
		return nil, err
	}

	python := &Python{ast: ast, scope: nil, source: sourceBytes}

	scope, scopeMap := makeLexicalScopeTree(python, ast)
	python.scope = scope
	python.scopeOfNode = scopeMap

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
				return []Decl{{lhs.Content(py.source), rhs}}
			} else if lhs.Type() == "pattern_list" && rhs.Type() == "expression_list" {
				decls := []Decl{}

				nLeftChildren := int(lhs.NamedChildCount())
				nRightChildren := int(rhs.NamedChildCount())

				for i := 0; i < min(nLeftChildren, nRightChildren); i++ {
					if lhs.NamedChild(i).Type() == "identifier" {
						decls = append(decls, Decl{
							lhs.NamedChild(i).Content(py.source),
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
				return []Decl{{funcName.Content(py.source), node}}
			}
			// TODO@(Srijan/Tushar) bind function parameters
		}
	}

	return nil
}

func (p *Python) GetScopeTree() (*Scope, ScopeOfNode) {
	return p.scope, p.scopeOfNode
}
