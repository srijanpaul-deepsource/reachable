package sniper

import sitter "github.com/smacker/go-tree-sitter"

type Module struct {
	Ast         *sitter.Node
	Source      []byte
	GlobalScope *Scope
	ScopeOfNode ScopeOfNode
	TsLanguage  *sitter.Language
}

type Language interface {
	Module() *Module
	GetDecls(*sitter.Node) []Decl
	IsCallExpr(*sitter.Node) bool
	IsFunctionDef(*sitter.Node) bool
	GetCalleeName(*sitter.Node) *string
	BodyOfFunction(*sitter.Node) *sitter.Node
	NameOfFunction(*sitter.Node) string
}
