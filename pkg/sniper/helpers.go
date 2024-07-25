package sniper

import (
	"github.com/emicklei/dot"
	sitter "github.com/smacker/go-tree-sitter"
)

func DotGraphFromTsQuery(queryStr string, lang Language) *dot.Graph {
	q, _ := sitter.NewQuery([]byte(queryStr), lang.Module().TsLanguage)
	qc := sitter.NewQueryCursor()
	qc.Exec(q, lang.Module().Ast)

	for {
		match, ok := qc.NextMatch()
		if !ok {
			break
		}

		match = qc.FilterPredicates(match, lang.Module().Source)
		cg := NewCallGraph(lang)
		if cg == nil {
			return nil
		}

		for _, c := range match.Captures {
			node := c.Node
			cgNode := cg.FindCallGraph(node)
			if cgNode == nil {
				return nil
			}
			graph := cgNode.ToDotGraph(cg)
			return graph
		}
	}

	return nil
}
