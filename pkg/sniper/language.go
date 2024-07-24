package sniper

import sitter "github.com/smacker/go-tree-sitter"

// Module represents a single parsed file of any language.
type Module struct {
	Ast         *sitter.Node
	FileName    string
	Source      []byte
	GlobalScope *Scope
	ScopeOfNode ScopeOfNode
	TsLanguage  *sitter.Language
}

// Language is a wrapper around a single parsed module
// of any supported language.
type Language interface {
	Module() *Module
	GetDecls(*sitter.Node) []Decl
	IsCallExpr(*sitter.Node) bool
	IsFunctionDef(*sitter.Node) bool
	GetCalleeName(*sitter.Node) *string
	BodyOfFunction(*sitter.Node) *sitter.Node
	NameOfFunction(*sitter.Node) string
}
