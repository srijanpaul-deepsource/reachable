package sniper

import (
	"github.com/emicklei/dot"
	sitter "github.com/smacker/go-tree-sitter"
	"github.com/srijanpaul-deepsource/reachable/pkg/util"
)

type CallExprWalker struct {
	callGraph *CallGraph
	file      ParsedFile
}

func (c *CallExprWalker) OnEnterNode(node *sitter.Node) bool {
	if node.Type() == "call" {
		c.callGraph.FindCallGraph(c.file, node)
	}

	return true
}

func (c *CallExprWalker) OnLeaveNode(node *sitter.Node) {
	// empty because we aren't interested in the exit event
}

func CallGraphFromFile(file ParsedFile, moduleCache map[string]ParsedFile) *CallGraph {
	cg := NewCallGraph()
	cg.ModuleCache = moduleCache
	cgWalker := &CallExprWalker{callGraph: cg, file: file}
	util.WalkTree(file.Module().Ast, cgWalker)

	return cg
}

func Cg2Dg(cg *CallGraph) *dot.Graph {
	graph := dot.NewGraph()
	visited := make(map[*CgNode]dot.Node)
	for _, cgNode := range cg.CallGraphOfNode {
		cgNode.ToDotNode(cg, graph, visited)
	}

	return graph
}

func DotGraphFromFile(file ParsedFile, moduleCache map[string]ParsedFile) *dot.Graph {
	cg := CallGraphFromFile(file, moduleCache)
	return Cg2Dg(cg)
}

func DotGraphFromTsQuery(queryStr string, file ParsedFile) *dot.Graph {
	q, _ := sitter.NewQuery([]byte(queryStr), file.Module().TsLanguage)
	qc := sitter.NewQueryCursor()
	qc.Exec(q, file.Module().Ast)

	for {
		match, ok := qc.NextMatch()
		if !ok {
			break
		}

		match = qc.FilterPredicates(match, file.Module().Source)
		cg := NewCallGraph()
		if cg == nil {
			return nil
		}

		for _, c := range match.Captures {
			node := c.Node
			cgNode := cg.FindCallGraph(file, node)
			if cgNode == nil {
				return nil
			}
			graph := cgNode.ToDotGraph(cg)
			return graph
		}
	}

	return nil
}
