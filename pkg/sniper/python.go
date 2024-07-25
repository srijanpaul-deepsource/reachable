package sniper

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	sitter "github.com/smacker/go-tree-sitter"
	treeSitterPy "github.com/smacker/go-tree-sitter/python"
)

type Python struct {
	module *Module
}

func (py *Python) Module() *Module {
	return py.module
}

// findProjectRoot tries to find the root project for any python file.
// It keeps traversing up the directory tree until it sees a "setup.py" or some
// other config file.
func findProjectRoot(filePath string) (*string, error) {
	// TODO(@Tushar/Srijan): Cache the filepaths for every directory we encounter
	absPath, err := filepath.Abs(filePath)
	if err != nil {
		return nil, err
	}

	for dir := filepath.Dir(absPath); dir != "/"; dir = filepath.Dir(dir) {
		filesInRoot := []string{"setup.py", "setup.cfg", "pyproject.toml"}

		for _, file := range filesInRoot {
			fullPath := filepath.Join(dir, file)
			_, err := os.Stat(fullPath)
			if err == nil { // file exists
				return &dir, nil
			}
		}
	}

	return nil, fmt.Errorf("could not find a parent directory with setup.py")
}

func ParsePython(fileName string, source string) (*Python, error) {
	sourceBytes := []byte(source)
	projectRoot, _ := findProjectRoot(fileName)

	python := &Python{module: &Module{
		FileName:    fileName,
		Source:      sourceBytes,
		ProjectRoot: projectRoot,
		TsLanguage:  treeSitterPy.GetLanguage(),
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

func (py *Python) NameOfFunction(node *sitter.Node) *string {
	if node.Type() == "function_definition" {
		name := node.ChildByFieldName("name").Content(py.module.Source)
		return &name
	}

	if node.Type() == "lambda" {
		nearestScope := GetScope(py.Module(), node)
		name := nearestScope.NameOfNode[node]
		return &name
	}

	return nil
}

func (py *Python) IsImport(node *sitter.Node) bool {
	return node.Type() == "import_from_statement" || node.Type() == "import_statement"
}

func (py *Python) FilePathOfImport(node *sitter.Node) *string {
	cached := py.module.FilePathOfImport[node]
	if cached != "" {
		return &cached
	}

	// TODO(@Tushar/Srijan): `import *` is not handled
	var moduleName string
	var itemName string
	if node.Type() == "import_from_statement" {
		moduleName = node.ChildByFieldName("module_name").Content(py.module.Source)
		upLevel := 0

		for strings.HasPrefix(moduleName, ".") {
			moduleName = moduleName[1:]
			upLevel++
		}

		// TODO: there's no `children_by_field_name` equivalent in go
		itemName = node.ChildByFieldName("name").Content(py.module.Source)

	} else if node.Type() == "import_statement" {
		// TODO
		return nil
	}

	// println("moduleName:", moduleName)
	baseModulePath := filepath.Join(strings.Split(moduleName, ".")...)

	modulePaths := []string{
		filepath.Join(baseModulePath, itemName),
		baseModulePath,
	}

	relPaths := []string{".", "src"}
	for _, relPath := range relPaths {
		for _, modulePath := range modulePaths {
			possibleFileA := filepath.Join(*py.Module().ProjectRoot, relPath, modulePath, "__init__.py")
			possibleFileB := filepath.Join(*py.Module().ProjectRoot, relPath, modulePath+".py")

			if _, err := os.Stat(possibleFileA); err == nil {
				py.module.FilePathOfImport[node] = possibleFileA
				return &possibleFileA
			}

			if _, err := os.Stat(possibleFileB); err == nil {
				py.module.FilePathOfImport[node] = possibleFileB
				return &possibleFileB
			}
		}
	}

	return nil
}
