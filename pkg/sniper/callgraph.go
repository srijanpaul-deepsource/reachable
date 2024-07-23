package sniper

import (
	"fmt"

	"github.com/emicklei/dot"
	sitter "github.com/smacker/go-tree-sitter"
)

type CgNode struct {
	Func     *sitter.Node
	Children []*CgNode
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
	if fn == nil {
		return nil
	}

	cgNode := cg.traverseFunction(fn, 0)
	cg.CallGraphOfNode[node] = cgNode
	return cgNode
}

func (cg *CallGraph) traverseFunction(fn *sitter.Node, funcDepth int) *CgNode {
	cached := cg.CallGraphOfNode[fn]
	if cached != nil {
		return cached
	}

	cgNode := &CgNode{Func: fn, Children: nil}
	cg.CallGraphOfNode[fn] = cgNode

	body := cg.language.BodyOfFunction(fn)

	for stmt := body.Child(0); stmt != nil; stmt = stmt.NextSibling() {
		if cg.language.IsFunctionDef(stmt) {
			cg.traverseFunction(stmt, funcDepth+1)
			continue
		}

		if cg.language.IsCallExpr(stmt) {
			fun := cg.resolveCallExpr(stmt)
			if fun == nil {
				continue
			}

			child := &CgNode{Func: fun, Children: nil}
			cg.traverseFunction(fun, 0)

			cgNode.Children = append(cgNode.Children, child)
		}
	}

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
	g := dot.NewGraph()
	cgNode.toDotNode(cg, g)
	return g
}

func (cgNode *CgNode) toDotNode(cg *CallGraph, g *dot.Graph) dot.Node {
	current := g.Node(fmt.Sprintf("%p", &cgNode))
	label := "(missing)"

	if cgNode.Func != nil {
		name := cg.language.NameOfFunction(cgNode.Func)
		if name != "" {
			label = name
		}
	}

	current = current.Attr("label", label)

	for _, child := range cgNode.Children {
		child := child.toDotNode(cg, g)
		g.Edge(current, child)
	}

	return current
}
