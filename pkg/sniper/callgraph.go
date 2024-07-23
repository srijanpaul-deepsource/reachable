package sniper

import sitter "github.com/smacker/go-tree-sitter"

type CgNode struct {
	caller *sitter.Node
	callee *sitter.Node
}

type CallGraph struct {
	language Module
	root     *CgNode
}

func NewCallGraph(language Module) *CallGraph {
	return &CallGraph{language: language}
}
