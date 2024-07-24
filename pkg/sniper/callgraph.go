package sniper

import (
	"fmt"

	"github.com/emicklei/dot"
	sitter "github.com/smacker/go-tree-sitter"
	"github.com/srijanpaul-deepsource/reachable/pkg/util"
)

type CgNode struct {
	Func      *sitter.Node
	Neighbors []*CgNode
}

type CallGraph struct {
	language        Language
	CallGraphOfNode map[*sitter.Node]*CgNode
}

func NewCallGraph(language Language) *CallGraph {
	return &CallGraph{language: language, CallGraphOfNode: make(map[*sitter.Node]*CgNode)}
}

func (cg *CallGraph) FindCallGraph(node *sitter.Node) *CgNode {
	if !cg.language.IsCallExpr(node) {
		return nil
	}

	cached, exists := cg.CallGraphOfNode[node]
	if exists {
		return cached
	}

	fn := cg.resolveCallExpr(node)
	// fmt.Printf("%s\n", fn.Content(cg.language.Module().Source))
	if fn == nil {
		return nil
	}

	cgNode := cg.traverseFunction(fn)
	cg.CallGraphOfNode[node] = cgNode
	return cgNode
}

type AstWalker struct {
	language      Language
	currentCgNode *CgNode
	cg            *CallGraph
}

func (walker *AstWalker) OnEnterNode(node *sitter.Node) bool {
	if walker.language.IsFunctionDef(node) {
		return false
	}

	if walker.language.IsCallExpr(node) {
		fun := walker.cg.resolveCallExpr(node)
		if fun == nil {
			return true
		}

		cgNode := walker.cg.traverseFunction(fun)
		walker.currentCgNode.Neighbors = append(walker.currentCgNode.Neighbors, cgNode)
		walker.cg.CallGraphOfNode[node] = cgNode
	}

	return true
}

func (walker *AstWalker) OnLeaveNode(node *sitter.Node) {
	// empty
}

func (cg *CallGraph) traverseFunction(fn *sitter.Node) *CgNode {
	cached := cg.CallGraphOfNode[fn]
	if cached != nil {
		return cached
	}

	cgNode := &CgNode{Func: fn, Neighbors: nil}
	cg.CallGraphOfNode[fn] = cgNode

	/* 	src := cg.language.Module().Source
	   	if string(src) == "" {
	   		// ???
	   	}
	*/
	walker := AstWalker{
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

func (cgNode *CgNode) ToDotGraph(cg *CallGraph) *dot.Graph {
	visited := make(map[*CgNode]dot.Node)
	g := dot.NewGraph()
	cgNode.toDotNode(cg, g, visited)
	return g
}

func (cgNode *CgNode) toDotNode(cg *CallGraph,
	g *dot.Graph,
	visited map[*CgNode]dot.Node,
) dot.Node {

	if cached, exists := visited[cgNode]; exists {
		return cached
	}

	current := g.Node(fmt.Sprintf("%p", &cgNode))
	label := "(missing)"

	if cgNode.Func != nil {
		name := cg.language.NameOfFunction(cgNode.Func)
		if name != "" {
			label = name
		}
	}

	current = current.Attr("label", label)
	visited[cgNode] = current

	for _, child := range cgNode.Neighbors {
		child := child.toDotNode(cg, g, visited)
		g.Edge(current, child)
	}

	return current
}
