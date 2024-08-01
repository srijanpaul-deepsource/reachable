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

// ChildrenWithFieldName returns all the children of a node
// with a specific field name.
// Tree-sitter can have multiple children with the same field name.
func ChildrenWithFieldName(node *sitter.Node, fieldName string) []*sitter.Node {
	children := []*sitter.Node{}
	for i := 0; i < int(node.ChildCount()); i++ {
		if node.FieldNameForChild(i) == fieldName {
			child := node.Child(i)
			children = append(children, child)
		}
	}

	return children
}

// FindMatchingChild iterates over all children of a node—both named and unnamed—and returns the
// first child that matches the predicate function.
func FindMatchingChild(node *sitter.Node, predicate func(*sitter.Node) bool) *sitter.Node {
	nChildren := int(node.ChildCount())

	for i := 0; i < nChildren; i++ {
		child := node.Child(i)
		if predicate(child) {
			return child
		}
	}

	return nil
}
