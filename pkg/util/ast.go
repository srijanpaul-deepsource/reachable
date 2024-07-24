package util

import (
	sitter "github.com/smacker/go-tree-sitter"
)

type Walker interface {
	OnEnterNode(node *sitter.Node) bool
	OnLeaveNode(node *sitter.Node)
}

func WalkTree(node *sitter.Node, walker Walker) {
	goInside := walker.OnEnterNode(node)

	if goInside {
		for i := 0; i < int(node.NamedChildCount()); i++ {
			child := node.NamedChild(i)
			WalkTree(child, walker)
		}

	}

	walker.OnLeaveNode(node)
}
