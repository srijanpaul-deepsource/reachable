package sniper

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	sitter "github.com/smacker/go-tree-sitter"
	treeSitterPy "github.com/smacker/go-tree-sitter/python"
	"github.com/srijanpaul-deepsource/reachable/pkg/util"
)

type Python struct {
	module           *Module
	SitePackagesPath string
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

	for dir := filepath.Dir(absPath); dir != "/"; {
		filesInRoot := []string{"setup.py", "setup.cfg", "pyproject.toml"}

		for _, file := range filesInRoot {
			fullPath := filepath.Join(dir, file)
			_, err := os.Stat(fullPath)
			if err == nil { // file exists
				return &dir, nil
			}
		}

		nextDir := filepath.Dir(dir)
		nextDirBaseName := filepath.Base(nextDir)
		if nextDirBaseName == "site-packages" {
			return &dir, nil
		}

		dir = filepath.Clean(nextDir)
	}

	return nil, fmt.Errorf("could not find a parent directory with setup.py")
}

func findVenvSitePackages(rootFolder string) (string, error) {
	files, err := os.ReadDir(rootFolder)
	if err != nil {
		return "", err
	}
	path, err := filepath.Abs(rootFolder)
	if err != nil {
		return "", err
	}
	for _, file := range files {
		if !file.IsDir() {
			continue
		}

		potentialVenvFolder := filepath.Join(path, file.Name())
		sitePackagesPaths, err := filepath.Glob(filepath.Join(potentialVenvFolder, "lib", "**", "site-packages"))
		if err != nil {
			return "", err
		}
		if len(sitePackagesPaths) == 0 {
			continue
		}
		return sitePackagesPaths[0], nil
	}

	return "", errors.New("no venv found")
}

func ParsePython(fileName string, source []byte) (*Python, error) {
	projectRoot, _ := findProjectRoot(fileName)
	python := &Python{
		module: &Module{
			FileName:         fileName,
			Source:           source,
			Language:         LangPy,
			ProjectRoot:      projectRoot,
			TsLanguage:       treeSitterPy.GetLanguage(),
			FilePathOfImport: make(map[*sitter.Node]string),
		},
	}

	codePath := os.Getenv("CODE_PATH")
	if codePath == "" {
		codePath, _ = os.Getwd()
	}
	sitePackagesPath, err := findVenvSitePackages(codePath)
	if err == nil {
		python.SitePackagesPath = sitePackagesPath
	}

	ast, err := sitter.ParseCtx(
		context.Background(), source, python.module.TsLanguage,
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
				// panic(fmt.Sprintf("lhs type: %s, rhs: %s", lhs.Type(), rhs.Type()))
				return nil
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

	case "class_definition":
		{
			className := node.ChildByFieldName("name")
			if className != nil {
				return []Decl{{className.Content(py.module.Source), node}}
			}
		}

	case "import_from_statement":
		{
			importedSymbols := util.ChildrenWithFieldName(node, "name")
			var decls []Decl
			for _, nameNode := range importedSymbols {
				switch nameNode.Type() {
				case "dotted_name":
					{
						firstChild := nameNode.Child(0)
						if firstChild.Type() == "identifier" {
							name := firstChild.Content(py.module.Source)
							decls = append(decls, Decl{name, node})
						}
					}
				case "aliased_import":
					{
						aliasNode := nameNode.ChildByFieldName("alias")
						name := aliasNode.Content(py.module.Source)
						decls = append(decls, Decl{name, node})
					}
				default:
					panic("Imported symbol is a " + nameNode.Type())
				}
			}

			return decls
		}

	case "import_statement":
		{
			// TODO: use children with field name and support multple.
			module := node.ChildByFieldName("name")
			if module.Type() == "dotted_name" {
				name := module.Content(py.module.Source)
				if strings.Contains(name, ".") {
					// TODO: import dotted import statements (and handle in callgraph)
				} else {
					return []Decl{{name, node}}
				}
			}
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

func (py *Python) IsModuleImport(node *sitter.Node) bool {
	return node.Type() == "import_statement"
}

func (py *Python) FilePathOfImport(node *sitter.Node) *string {
	cached := py.module.FilePathOfImport[node]
	if cached != "" {
		return &cached
	}

	// TODO(@Tushar/Srijan): `import *` is not handled
	var moduleName string
	var itemName string
	upLevel := 0
	if node.Type() == "import_from_statement" {
		moduleName = node.ChildByFieldName("module_name").Content(py.module.Source)
		for strings.HasPrefix(moduleName, ".") {
			moduleName = moduleName[1:]
			upLevel++
		}

		// TODO: there are multiple `name` field here, right now we just use the
		// first one. refer to the TODO in `language.go` about `FilePathOfImport()`,
		// we should build a map of import node to filepath instead.
		// To do that we can use the ChildrenWithFieldName helper here.
		itemName = node.ChildByFieldName("name").Content(py.module.Source)

	} else if node.Type() == "import_statement" {
		moduleName = node.ChildByFieldName("name").Content(py.module.Source)
		for strings.HasPrefix(moduleName, ".") {
			moduleName = moduleName[1:]
			upLevel++
		}

		itemName = ""
	}

	baseModulePath := filepath.Join(strings.Split(moduleName, ".")...)

	relPaths := []string{".", "src"}
	var rootPath string
	if upLevel > 0 {
		rootPath = py.Module().FileName
		for i := 0; i < upLevel; i++ {
			rootPath = filepath.Dir(rootPath)
		}
	} else {
		rootPath = *py.Module().ProjectRoot
	}
	modulePaths := []string{baseModulePath}
	if itemName != "" {
		modulePaths = append(modulePaths, filepath.Join(baseModulePath, itemName))
	}

	for _, relPath := range relPaths {
		for _, modulePath := range modulePaths {
			// fmt.Fprintf(os.Stderr, "%v  %v  %v\n", py.SitePackagesPath, relPath, modulePath)
			possibleFiles := []string{
				filepath.Join(rootPath, relPath, modulePath, "__init__.py"),
				filepath.Join(rootPath, relPath, modulePath+".py"),
			}
			if py.SitePackagesPath != "" {
				possibleFiles = append(possibleFiles,
					filepath.Join(py.SitePackagesPath, relPath, modulePath, "__init__.py"),
					filepath.Join(py.SitePackagesPath, relPath, modulePath+".py"),
				)
			}
			for _, possibleFile := range possibleFiles {
				if _, err := os.Stat(possibleFile); err == nil {
					py.module.FilePathOfImport[node] = possibleFile
					return &possibleFile
				}
			}
		}
	}

	return nil
}

func (py *Python) IsDottedExpr(node *sitter.Node) bool {
	return node.Type() == "attribute"
}

func (py *Python) GetCallee(callExpr *sitter.Node) *sitter.Node {
	return callExpr.ChildByFieldName("function")
}

func (py *Python) FunctionDefFromNode(node *sitter.Node) *sitter.Node {
	if node.Type() != "class_definition" {
		return nil
	}

	// TODO: super calls?
	body := node.ChildByFieldName("body")
	// TODO: make this a reverse iterator instead to find the last __init__
	// because @overload's exist
	return util.FindMatchingChild(body, func(child *sitter.Node) bool {
		if child.Type() != "function_definition" {
			return false
		}

		funcName := child.ChildByFieldName("name")
		if funcName == nil {
			return false
		}

		return funcName.Content(py.module.Source) == "__init__"
	})
}

func (py *Python) ResolveExportedSymbol(name string) *sitter.Node {
	globalScope := py.Module().GlobalScope
	return globalScope.Symbols[name]
}

func (py *Python) GetObjectAndProperty(node *sitter.Node) (*sitter.Node, *sitter.Node) {
	return node.ChildByFieldName("object"), node.ChildByFieldName("attribute")
}

func (py *Python) PackageName() *string {
	if py.module.ProjectRoot == nil {
		return nil
	}

	baseName := filepath.Base(*py.module.ProjectRoot)
	return &baseName
}
