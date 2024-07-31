package sniper

import (
	"fmt"
	"path"
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

func NewCgNode(file ParsedFile, fn *sitter.Node) *CgNode {
	var funcName *string
	if fn != nil {
		funcName = file.NameOfFunction(fn)
	}
	return &CgNode{Func: fn, FuncName: funcName, File: file}
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
	fn := cg.resolveCallExpr(file, node)
	if fn == nil {
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

	if file.IsImport(fn) {
		// 1. Resolve the import to a file path
		// 2. Parse the file into a Language.Module struct
		// 3. Find the function definition in the module that the import resolves to
		// 4. Create a call graph for that node.

		// Resolve the import to a file.
		filePath := file.FilePathOfImport(fn)
		if filePath == nil {
			return nil
		}

		// Check if the module is already importedFile
		importedFile, exists := cg.ModuleCache[*filePath]
		if !exists {
			var err error
			importedFile, err = ParseFile(file.Module().Language, *filePath)
			if err != nil {
				// TODO: return error when file parse fails.
				return nil
			}
			cg.ModuleCache[*filePath] = importedFile
		}

		// Find the function definition in the module
		nameOfCallee := file.GetCalleeName(node)
		if nameOfCallee == nil {
			return nil
		}

		def := importedFile.ResolveExportedSymbol(*nameOfCallee)
		if def == nil {
			return nil
		}

		if importedFile.IsImport(def) {
			// TODO(@Tushar/Srijan): handle re-exports
			return nil
		}

		if importedFile.IsFunctionDef(def) {
			cgNode := cg.traverseFunction(importedFile, def)
			cg.CallGraphOfNode[node] = cgNode
			return cgNode
		} else {
			// TODO: what cases did we not handle?
			return nil
		}
	}

	// Traverse the body of that function, and create the call-graph.
	cgNode := cg.traverseFunction(file, fn)
	cg.CallGraphOfNode[node] = cgNode
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
	cached := cg.CallGraphOfNode[fn]
	if cached != nil {
		return cached
	}

	cgNode := NewCgNode(file, fn)
	cg.CallGraphOfNode[fn] = cgNode

	walker := callExprWalker{
		file:          file,
		currentCgNode: cgNode,
		cg:            cg,
	}

	body := file.BodyOfFunction(fn)
	util.WalkTree(body, &walker)

	return cgNode
}

// resolveCallExpr takes a call expression node, and
// returns the function definition for the callee.
func (cg *CallGraph) resolveCallExpr(file ParsedFile, callExpr *sitter.Node) *sitter.Node {
	scope := GetScope(file.Module(), callExpr)
	if scope == nil {
		return nil
	}

	name := file.GetCalleeName(callExpr)
	if name == nil {
		return nil
	}

	decl := scope.Lookup(*name)
	return decl
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

	fileName := cgNode.File.Module().FileName
	fileName = strings.TrimSuffix(fileName, path.Ext(fileName))

	current := g.Node(fmt.Sprintf("%p", &cgNode))
	label := fileName + ":(unresolved)"

	if cgNode.FuncName != nil {
		if cgNode.Func == nil {
			// If the function body could not be resolved,
			// we do not know which module it comes from.
			label = "(unresolved):" + *cgNode.FuncName
		} else {
			label = fileName + ":" + *cgNode.FuncName
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
