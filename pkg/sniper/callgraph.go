package sniper

import (
	"fmt"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"github.com/emicklei/dot"
	sitter "github.com/smacker/go-tree-sitter"
	"github.com/srijanpaul-deepsource/reachable/pkg/util"
)

// CgNode is a node in the call-graph
type CgNode struct {
	// Func is the function declaration for this node
	// This is nil for builtins like `print`.
	Func *sitter.Node
	// Name of the function being called
	FuncName *string
	// Neighbors is a list of CgNodes for other functions that are called
	// inside the body of `Func`
	Neighbors []*CgNode
	// The file that this call-graph node belongs to
	File ParsedFile
}

func NewCgNode(file ParsedFile, fn *sitter.Node) CgNode {
	var funcName *string
	if fn != nil {
		funcName = file.NameOfFunction(fn)
	}
	return CgNode{Func: fn, FuncName: funcName, File: file}
}

// CallGraph maps a function definition or call-expression AST node
// to its corresponding call graph.
type CallGraph struct {
	CallGraphOfNode map[*sitter.Node]*CgNode
	// Stub Call-graph nodes (leaves) for unresolved functions.
	// Usually, this is a builtin (like `print` in python).
	// TODO: what about methods? `os.exec()`?
	UnresolvedCgNodes map[string]*CgNode
	ModuleCache       map[string]ParsedFile
}

// NewCallGraph creates an empty call graph
func NewCallGraph() *CallGraph {
	return &CallGraph{
		CallGraphOfNode:   make(map[*sitter.Node]*CgNode),
		UnresolvedCgNodes: make(map[string]*CgNode),
	}
}

// FindCallGraph finds a call-graph corresponding to a call-expression node.
func (cg *CallGraph) FindCallGraph(file ParsedFile, node *sitter.Node) *CgNode {
	if !file.IsCallExpr(node) {
		// We do not resolve
		return nil
	}

	// Check if a cached call-graph exists
	cached, exists := cg.CallGraphOfNode[node]
	if exists {
		return cached
	}

	// If not, find the function that the call-expression is calling.
	// TODO(@Tushar/Srijan): Make this work for methods and not just identifiers
	resolvedCallExpr := cg.resolveCallExpr(file, node)

	// TODO: resolve a single call node to multiple call graphs.
	// This is because, a single call expression can resolve to multiple function defs.
	// Eg:
	// ```python
	// f = lambda x : x - 1
	// if condition:
	// 	 f = lambda x : x + 1
	// y = f(1) // <- `f` can be either of the lambdas.
	// ```
	if len(resolvedCallExpr) == 0 {
		return nil
	}

	nextFile, nextNode := resolvedCallExpr[0].File, resolvedCallExpr[0].Node
	if nextNode == nil {
		calleeName := file.GetCalleeName(node)
		if calleeName != nil {
			cgNode, exists := cg.UnresolvedCgNodes[*calleeName]
			if exists {
				return cgNode
			}
		}

		cgNode := &CgNode{FuncName: calleeName, File: file}
		if calleeName != nil {
			cg.UnresolvedCgNodes[*calleeName] = cgNode
		}
		return cgNode
	}

	if nextFile.IsImport(nextNode) {
		calleeName := nextFile.GetCalleeName(node)
		if calleeName == nil {
			return nil
		}

		cgNode := cg.cgNodeFromImport(nextFile, nextNode, *calleeName)
		if cgNode != nil {
			cg.CallGraphOfNode[node] = cgNode
		}
		return cgNode
	}

	// Traverse the body of that function, and create the call-graph.
	cgNode := cg.traverseFunction(nextFile, nextNode)
	return cgNode
}

// callExprWalker is an AST walker for walking function bodies
// and building the call graph nodes for every call-expr
// in there.
type callExprWalker struct {
	file          ParsedFile
	currentCgNode *CgNode
	cg            *CallGraph
}

// OnEnterNode is needed by `Walker` interface
func (walker *callExprWalker) OnEnterNode(node *sitter.Node) bool {
	// Do not enter nested functions.
	// Eg:
	// function f() {
	//  var g = function () {
	//     h()
	//  }
	//  return g
	// }
	// The call graph for the above should be:
	// f -> <end>
	// And not:
	// f -> h
	if walker.file.IsFunctionDef(node) {
		return false
	}

	if walker.file.IsCallExpr(node) {
		cgNode := walker.cg.FindCallGraph(walker.file, node)
		if cgNode == nil {
			return true
		}

		walker.currentCgNode.Neighbors = append(walker.currentCgNode.Neighbors, cgNode)
		walker.cg.CallGraphOfNode[node] = cgNode
	}

	return true
}

// OnLeaveNode is needed by `Walker` interface
func (walker *callExprWalker) OnLeaveNode(node *sitter.Node) {
	// empty
}

// traverseFunction traverses the body of `fn` (a function def node)
// and builds a CgNode where the root node is `fn`
func (cg *CallGraph) traverseFunction(file ParsedFile, fn *sitter.Node) *CgNode {
	cachedNode, cached := cg.CallGraphOfNode[fn]
	if cached {
		return cachedNode
	}

	cgNode := NewCgNode(file, fn)
	cg.CallGraphOfNode[fn] = &cgNode

	walker := callExprWalker{
		file:          file,
		currentCgNode: &cgNode,
		cg:            cg,
	}

	body := file.BodyOfFunction(fn)
	util.WalkTree(body, &walker)

	return &cgNode
}

// ResolvedExpr contains the node that an expression "resolves" to.
// See below.
type ResolvedExpr struct {
	// Node is the AST node that the expression resolves to.
	Node *sitter.Node
	// File is the file that the node belongs to
	File ParsedFile
}

// resolveExpr resolves an arbitrary expression to class/function declarations
// (there can be multiple)
// e.g, In this snippet:
// ```py
// def foo(): ...
// bar = foo
// class A: ...
// bar = A
// ```
// The identifier "bar" will be resolved to the list that contains
// the nodes [`def foo(): ...`, `class A: ...`],
func (cg *CallGraph) resolveExpr(file ParsedFile, node *sitter.Node) []ResolvedExpr {
	var results []ResolvedExpr

	// scanIdentifier resolves an identifier to its declaration nodes
	// (and files, if its imported), and then appends them to the results.
	// Returns `true` if the identifier could be resolved to at least one declaration.
	scanIdentifier := func(file ParsedFile, id *sitter.Node) bool {
		resolved := cg.resolveIdentifier(file, id)
		success := false

		for _, resolvedNode := range resolved {
			if file.IsFunctionDef(resolvedNode) {
				results = append(results, ResolvedExpr{
					Node: resolvedNode,
					File: file,
				})
				success = true
			} else if file.IsImport(resolvedNode) {
				// identifier resolved to an import statement.
				nameOfId := id.Content(file.Module().Source)
				importedFile, nodeInImportedFile := cg.resolveImport(file, resolvedNode, nameOfId)

				if importedFile != nil && nodeInImportedFile != nil {
					results = append(results, ResolvedExpr{File: importedFile, Node: nodeInImportedFile})
					success = true
				}
			} else {
				funDef := file.FunctionDefFromNode(resolvedNode)
				if funDef != nil {
					results = append(results, ResolvedExpr{
						Node: funDef,
						File: file,
					})
					success = true
				}
			}
		}

		return success
	}

	if file.IsDottedExpr(node) {
		// It's possible for a dotted expr to also be in the lookup table.
		// This is because python imports are weird.
		if !scanIdentifier(file, node) {
			resolvedDottedExprs := cg.resolveDottedExpr(file, node)
			for _, resolved := range resolvedDottedExprs {
				nextFile, nextNode := resolved.File, resolved.Node
				str := nextNode.Content(nextFile.Module().Source)
				_ = str
				results = append(results, cg.resolveExpr(nextFile, nextNode)...)
			}
		}
	} else if node.Type() == "identifier" {
		scanIdentifier(file, node)
	} else if file.IsFunctionDef(node) {
		results = append(results, ResolvedExpr{File: file, Node: node})
	} else {
		funDef := file.FunctionDefFromNode(node)
		if funDef != nil {
			results = append(results, ResolvedExpr{File: file, Node: funDef})
		}
	}

	return results
}

// resolveIdentifier resolves an identifier to the list of expressions
// that were assigned to it *within* the same file.
func (cg *CallGraph) resolveIdentifier(file ParsedFile, idNode *sitter.Node) []*sitter.Node {
	module := file.Module()

	// 1. Find where this identifier was declared.
	scope := GetScope(module, idNode)
	if scope == nil {
		return nil
	}

	// TODO(@srijan/tushar): check for infinite loops
	initExprs := scope.Lookup(idNode.Content(module.Source))
	return initExprs
}

// TODO: test this very very very thoroughly

// resolveDottedExpr takes a dotted expression node, and returns
// the function definition or class/object node that it is bound to (if any could be found).
func (cg *CallGraph) resolveDottedExpr(file ParsedFile, dottedExpr *sitter.Node) []ResolvedExpr {
	object, property := file.GetObjectAndProperty(dottedExpr)
	if object == nil || property == nil || property.Type() != "identifier" {
		return nil
	}

	var results []ResolvedExpr

	resolvedExprs := cg.resolveExpr(file, object)
	for _, resolved := range resolvedExprs {
		nextFile, def := resolved.File, resolved.Node
		if nextFile == nil || def == nil {
			continue
		}
		if !slices.Contains(ScopeNodeTypes, def.Type()) {
			continue
		}

		scope := nextFile.Module().ScopeOfNode[def]
		if scope == nil {
			continue
		}

		propName := property.Content(file.Module().Source)
		decls, exists := scope.Symbols[propName]
		if !exists {
			continue
		}

		for _, decl := range decls {
			if nextFile.IsImport(decl) {
				importedFile, importedExpr := cg.resolveImport(nextFile, decl, propName)
				if importedFile != nil && importedExpr != nil {
					results = append(results, ResolvedExpr{
						File: importedFile,
						Node: importedExpr,
					})
				}
			} else {
				results = append(results, ResolvedExpr{
					File: nextFile,
					Node: decl,
				})
			}

		}
	}

	return results
}

// resolveCallExpr takes a call expression node, and
// returns the function definition for the callee.
func (cg *CallGraph) resolveCallExpr(file ParsedFile, callExpr *sitter.Node) []ResolvedExpr {
	scope := GetScope(file.Module(), callExpr)
	if scope == nil {
		return nil
	}

	callee := file.GetCallee(callExpr)
	if callee == nil {
		return nil
	}

	var results []ResolvedExpr
	resolvedExprs := cg.resolveExpr(file, callee)

	for _, resolved := range resolvedExprs {
		file, decl := resolved.File, resolved.Node
		if file.IsFunctionDef(decl) || file.IsImport(decl) {
			results = append(results, resolved)
		} else {
			defNode := file.FunctionDefFromNode(decl)
			if defNode != nil {
				results = append(results, ResolvedExpr{
					File: file,
					Node: defNode,
				})
			}
		}
	}

	return results
}

// ToDotGraph converts a CallGraph to a dot graph for debugging/visualization
func (cgNode *CgNode) ToDotGraph(cg *CallGraph) *dot.Graph {
	visited := make(map[*CgNode]dot.Node)
	g := dot.NewGraph()
	cgNode.ToDotNode(cg, g, visited)
	return g
}

// ToDotNode converts a CgNode to a dot graph node.
func (cgNode *CgNode) ToDotNode(cg *CallGraph,
	g *dot.Graph,
	visited map[*CgNode]dot.Node,
) dot.Node {
	if cached, exists := visited[cgNode]; exists {
		return cached
	}

	filePath := cgNode.File.Module().FileName
	if cgNode.File.Module().ProjectRoot != nil {
		relFilePath, err := filepath.Rel(*cgNode.File.Module().ProjectRoot, filePath)
		if err == nil {
			projectName := filepath.Base(*cgNode.File.Module().ProjectRoot)
			filePath = projectName + "::" + relFilePath
		}
	}
	filePath = strings.TrimSuffix(filePath, path.Ext(filePath))

	current := g.Node(fmt.Sprintf("%p", &cgNode))
	label := filePath + ":(unresolved)"

	if cgNode.FuncName != nil {
		if cgNode.Func == nil {
			// If the function body could not be resolved,
			// we do not know which module it comes from.
			label = "(unresolved):" + *cgNode.FuncName
		} else {
			label = filePath + ":" + *cgNode.FuncName
		}
	}

	current = current.Label(label)
	visited[cgNode] = current

	for _, neighbor := range cgNode.Neighbors {
		newNode := neighbor.ToDotNode(cg, g, visited)
		g.Edge(current, newNode)
	}

	return current
}

type WalkFn func(*CgNode, []*CgNode)

func (callGraph *CallGraph) Walk(fromFile string, visitFn WalkFn) {
	visited := make(map[*CgNode]struct{})

	fromFile, err := filepath.Abs(fromFile)
	if err != nil {
		panic(err)
	}

	var path []*CgNode
	for _, root := range callGraph.CallGraphOfNode {
		if root == nil {
			panic("impossible")
		}

		if root.File.Module().FileName != fromFile {
			continue
		}

		if _, alreadyVisited := visited[root]; !alreadyVisited {
			path = append(path, root)
			root.walk(visited, &path, visitFn)
			path = path[:len(path)-1]
		}
	}
}

func (cgNode *CgNode) walk(visited map[*CgNode]struct{}, path *[]*CgNode, fn WalkFn) {
	*path = append(*path, cgNode)
	fn(cgNode, *path)

	for _, neighbor := range cgNode.Neighbors {
		if neighbor == nil {
			panic("impossible")
		}

		if _, alreadyVisited := visited[neighbor]; !alreadyVisited {
			visited[neighbor] = struct{}{}
			neighbor.walk(visited, path, fn)
		}
	}

	*path = (*path)[:len(*path)-1]
}

func (cg *CallGraph) cgNodeFromImport(file ParsedFile, defNode *sitter.Node, calleeName string) *CgNode {
	importedFile, defInImportedFile := cg.resolveImport(file, defNode, calleeName)
	return cg.traverseFunction(importedFile, defInImportedFile)
}

func (cg *CallGraph) resolveImport(file ParsedFile, importStmt *sitter.Node, calleeName string) (ParsedFile, *sitter.Node) {
	// 1. Resolve the import to a file path
	// 2. Parse the file into a Language.Module struct
	// 3. Find the function definition in the module that the import resolves to
	// 4. Create a call graph for that node.

	// Resolve the import to a file.
	filePath := file.FilePathOfImport(importStmt)
	if filePath == nil {
		return nil, nil
	}

	// Check if the module is already importedFile
	importedFile, exists := cg.ModuleCache[*filePath]
	if !exists {
		var err error
		importedFile, err = ParseFile(file.Module().Language, *filePath)
		if err != nil {
			// TODO: return error when file parse fails.
			return nil, nil
		}
		cg.ModuleCache[*filePath] = importedFile
	}

	if file.IsModuleImport(importStmt) {
		return importedFile, importedFile.Module().Ast
	}

	// Find the function definition in the module
	if originalName := file.NameOfAliasedImport(calleeName); originalName != nil {
		calleeName = *originalName
	}

	def := importedFile.ResolveExportedSymbol(calleeName)
	if def == nil {
		return nil, nil
	}

	// TODO: what if its an expr?
	// e.g: foo = "bar" # py
	// e.g: export const foo = "bar" // js
	// in this case, also handle recursive imports :<
	if importedFile.IsImport(def) {
		return cg.resolveImport(importedFile, def, calleeName)
	}

	// TODO: check for infinite loops
	if !importedFile.IsFunctionDef(def) {
		def = importedFile.FunctionDefFromNode(def)
	}

	return importedFile, def
}
