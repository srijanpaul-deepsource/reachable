package sniper

import (
	"slices"

	sitter "github.com/smacker/go-tree-sitter"
)

var blockNodeTypes = []string{
	"module",
	"function_definition",
	"class_definition",
	"class_declaration",
	"method_definition",
	"function_declaration",
}

type ScopeOfNode map[*sitter.Node]*Scope

type Decl struct {
	Name     string
	InitExpr *sitter.Node
}

type Scope struct {
	Parent           *Scope
	Children         []*Scope
	Symbols          map[string]*sitter.Node
	ImportStatements map[string]*sitter.Node
	AstNode          *sitter.Node
}

// Lookup finds a symbol starting from the current scope and going up
func (s *Scope) Lookup(name string) *sitter.Node {
	if s.Symbols[name] != nil {
		return s.Symbols[name]
	}

	if s.Parent != nil {
		return s.Parent.Lookup(name)
	}

	return nil
}

type Language interface {
	GetTreeSitterLanguage() *sitter.Language
	GetScopeTree() (*Scope, ScopeOfNode)
	GetDecls(*sitter.Node) []Decl
}

// makeLexicalScopeTree generates a scope tree from the AST, along with a mapping from
// block nodes to scope objects
func makeLexicalScopeTree(lang Language, root *sitter.Node) (*Scope, ScopeOfNode) {
	globalScope := &Scope{
		Parent:           nil,
		AstNode:          root,
		Symbols:          make(map[string]*sitter.Node),
		ImportStatements: make(map[string]*sitter.Node),
	}
	scopeOfNode := make(ScopeOfNode)
	scopeOfNode[root] = globalScope

	makeLexicalScopeTree_(lang, root, globalScope, scopeOfNode)
	return globalScope, scopeOfNode
}

func makeLexicalScopeTree_(
	lang Language,
	node *sitter.Node,
	scope *Scope,
	scopeOfNode ScopeOfNode,
) {
	nodeType := node.Type()
	isBlockNode := slices.Contains(blockNodeTypes, nodeType)

	decls := lang.GetDecls(node)
	for _, decl := range decls {
		writeExpr, name := decl.InitExpr, decl.Name
		if writeExpr != nil && scope.Symbols[name] == nil {
			// add a new variable declaration to the scope
			// if it doesn't exist already
			scope.Symbols[name] = writeExpr
		}
	}

	nextScope := scope
	if isBlockNode {
		nextScope = &Scope{
			AstNode:          node,
			Parent:           scope,
			Symbols:          make(map[string]*sitter.Node),
			ImportStatements: make(map[string]*sitter.Node),
		}
		nextScope.Parent = scope
		scopeOfNode[node] = nextScope
		scope.Children = append(scope.Children, nextScope)
	}

	for i := 0; i < int(node.NamedChildCount()); i++ {
		child := node.NamedChild(i)
		makeLexicalScopeTree_(lang, child, nextScope, scopeOfNode)
	}
}

// findNearestBlockNode finds the nearest surrounding block node for any AST node
func findNearestBlockNode(node *sitter.Node) *sitter.Node {
	for node != nil {
		if slices.Contains(blockNodeTypes, node.Type()) {
			return node
		}
		node = node.Parent()
	}
	return nil
}

// GetScope finds the nearest surrounding scope of a node
func GetScope(l Language, node *sitter.Node) *Scope {
	_, scopeOfNode := l.GetScopeTree()

	nearestBlock := findNearestBlockNode(node)
	if nearestBlock == nil {
		return nil
	}

	nearestScope, exists := scopeOfNode[nearestBlock]
	if !exists {
		return nil
	}

	return nearestScope
}

func ChildOfType(node *sitter.Node, typeName string) *sitter.Node {
	for i := 0; i < int(node.NamedChildCount()); i++ {
		child := node.NamedChild(i)
		if child.Type() == typeName {
			return child
		}
	}
	return nil
}
