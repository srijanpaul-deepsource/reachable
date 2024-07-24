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
	Func *sitter.Node
	// Neighbors is a list of CgNodes for other functions that are called
	// inside the body of `Func`
	Neighbors []*CgNode
}

// CallGraph maps a function definition or call-expression AST node
// to its corresponding call graph.
type CallGraph struct {
	language        Language
	CallGraphOfNode map[*sitter.Node]*CgNode
}

// NewCallGraph creates an empty call graph
func NewCallGraph(language Language) *CallGraph {
	return &CallGraph{language: language, CallGraphOfNode: make(map[*sitter.Node]*CgNode)}
}

// FindCallGraph finds a call-graph corresponding to a call-expression node.
func (cg *CallGraph) FindCallGraph(node *sitter.Node) *CgNode {
	if !cg.language.IsCallExpr(node) {
		// We do not resolve
		return nil
	}

	// Check if a cached call-graph exists
	cached, exists := cg.CallGraphOfNode[node]
	if exists {
		return cached
	}

	// If not, find the function that the call-expression
	// is calling.
	// TODO(@Tushar/Srijan): Make this work for methods and not just identifiers
	fn := cg.resolveCallExpr(node)
	if fn == nil {
		return nil
	}

	// Traverse the body of that function, and create the call-graph.
	cgNode := cg.traverseFunction(fn)
	cg.CallGraphOfNode[node] = cgNode
	return cgNode
}

// CgAstWalker is an AST walker for walking function bodies
// and building the call graph nodes for every call-expr
// in there.
type CgAstWalker struct {
	language      Language
	currentCgNode *CgNode
	cg            *CallGraph
}

// OnEnterNode is needed by `Walker` interface
func (walker *CgAstWalker) OnEnterNode(node *sitter.Node) bool {
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
	if walker.language.IsFunctionDef(node) {
		return false
	}

	if walker.language.IsCallExpr(node) {
		cgNode := walker.cg.FindCallGraph(node)
		walker.currentCgNode.Neighbors = append(walker.currentCgNode.Neighbors, cgNode)
		walker.cg.CallGraphOfNode[node] = cgNode
	}

	return true
}

// OnLeaveNode is needed by `Walker` interface
func (walker *CgAstWalker) OnLeaveNode(node *sitter.Node) {
	// empty
}

// traverseFunction traverses the body of `fn` (a function def node)
// and builds a CgNode where the root node is `fn`
func (cg *CallGraph) traverseFunction(fn *sitter.Node) *CgNode {
	cached := cg.CallGraphOfNode[fn]
	if cached != nil {
		return cached
	}

	cgNode := &CgNode{Func: fn, Neighbors: nil}
	cg.CallGraphOfNode[fn] = cgNode

	walker := CgAstWalker{
		language:      cg.language,
		currentCgNode: cgNode,
		cg:            cg,
	}

	body := cg.language.BodyOfFunction(fn)
	util.WalkTree(body, &walker)

	return cgNode
}

// resolveCallExpr takes a call expression node, and
// returns the function definition for the callee.
func (cg *CallGraph) resolveCallExpr(callExpr *sitter.Node) *sitter.Node {
	scope := GetScope(cg.language.Module(), callExpr)
	if scope == nil {
		return nil
	}

	name := cg.language.GetCalleeName(callExpr)
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
	cgNode.toDotNode(cg, g, visited)
	return g
}

// toDotNode converts a CgNode to a dot graph node.
func (cgNode *CgNode) toDotNode(cg *CallGraph,
	g *dot.Graph,
	visited map[*CgNode]dot.Node,
) dot.Node {

	if cached, exists := visited[cgNode]; exists {
		return cached
	}

	fileName := cg.language.Module().FileName
	fileName = strings.TrimSuffix(fileName, path.Ext(fileName))

	current := g.Node(fmt.Sprintf("%p", &cgNode))
	label := fileName + ":(missing)"

	if cgNode.Func != nil {
		name := cg.language.NameOfFunction(cgNode.Func)
		if name != "" {
			label = fileName + ":" + name
		}
	}

	current = current.Label(label)
	visited[cgNode] = current

	for _, child := range cgNode.Neighbors {
		child := child.toDotNode(cg, g, visited)
		g.Edge(current, child)
	}

	return current
}
