package sniper

import (
	sitter "github.com/smacker/go-tree-sitter"
)

// Module represents a single parsed file of any language.
type Module struct {
	Ast *sitter.Node
	// ProjectRoot is the root directory of the project to which
	// this module belongs
	ProjectRoot      *string
	FileName         string
	Source           []byte
	GlobalScope      *Scope
	ScopeOfNode      ScopeOfNode
	TsLanguage       *sitter.Language
	FilePathOfImport map[*sitter.Node]string
	Language         Language
}

type Language int

const (
	LangError Language = iota
	LangPy
	LangJs
)

// ParsedFile is a wrapper around a single parsed module
// of any supported language.
type ParsedFile interface {
	// Module returns the language specific module
	Module() *Module

	// GetDecls reutrns a list of all declarations made in *sitter.Node
	// (does not traverse blocks and such, the argument should be an assignment node)
	GetDecls(*sitter.Node) []Decl
	// IsCallExpr returns `true` if the node is a call expression
	IsCallExpr(*sitter.Node) bool
	// IsFunctionDef returns `true` if the node is a function definition/expression
	IsFunctionDef(*sitter.Node) bool
	// GetCalleeName returns the name of the callee in a function call node
	GetCalleeName(*sitter.Node) *string

	// BodyOfFunction returns the body (e.g list of stmts) of a function node.
	BodyOfFunction(*sitter.Node) *sitter.Node
	// NameOfFunction returns the name of a function definition or lambda function
	NameOfFunction(*sitter.Node) *string
	// IsImport returns `true` if the node is a import statement (or expression, in some languages)
	IsImport(*sitter.Node) bool
	// FilePathOfImport resolves an import statement node to an absolute file path
	// Will return an empty string when resolution fails
	FilePathOfImport(*sitter.Node) *string
	// ResolveExportedSymbol resolves an exported symbol to its definition node
	ResolveExportedSymbol(string) *sitter.Node
}
