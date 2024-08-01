package sniper

import (
	"fmt"
	"slices"
	"strings"

	"github.com/emicklei/dot"
	sitter "github.com/smacker/go-tree-sitter"
)

var ScopeNodeTypes = []string{
	"module",
	"function_definition",
	"class_definition",
	"class_declaration",
	"method_definition",
	"function_declaration",
}

// ScopeOfNode maps a tree-sitter node (like
// function def or block node) to the scope
// that the node introduces into the program.
type ScopeOfNode map[*sitter.Node]*Scope

// Decl is any function or variable declaration
// found in the AST.
type Decl struct {
	// Name is the symbol of the declaration
	Name string
	// InitExpr is the expression node that a symbol
	// initialized to.
	InitExpr *sitter.Node
}

// Scope represents a single block or function
// scope in the AST.
type Scope struct {
	// Parent is the immediately upper scope
	Parent *Scope
	// Children is a list of sub-scopes
	Children []*Scope
	// Symbols maps a name to an AST node that
	// the name was initialized to
	Symbols map[string]*sitter.Node
	// Name is the inverse-map of `Symbols`.
	NameOfNode map[*sitter.Node]string
	// (TODO)
	FilePathOfImport map[*sitter.Node]string
	// AstNode is the node that introduced this scope
	// in the program
	AstNode *sitter.Node
}

// ToDotGraph generates a dot graph from the Scope
func (s *Scope) ToDotGraph() *dot.Graph {
	g := dot.NewGraph()
	s.toDotNode(g)
	return g
}

// toDotNode creates a dot-graph node from a scope node
func (s *Scope) toDotNode(g *dot.Graph) dot.Node {
	current := g.Node(fmt.Sprintf("%p", &s))

	labels := []string{}
	for k := range s.Symbols {
		labels = append(labels, k)
	}

	if len(labels) == 0 {
		labels = []string{"(empty)"}
	}

	current = current.Attr("label", strings.Join(labels, ", "))

	for _, child := range s.Children {
		child := child.toDotNode(g)
		g.Edge(current, child)
	}

	return current
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

// makeLexicalScopeTree generates a scope tree from the AST, along with a mapping from
// block nodes to scope objects
func makeLexicalScopeTree(lang ParsedFile, root *sitter.Node) (*Scope, ScopeOfNode) {
	scopeOfNode := make(ScopeOfNode)
	globalScope := makeLexicalScopeTree_(lang, root, nil, scopeOfNode)
	return globalScope, scopeOfNode
}

// makeLexicalScopeTree_ is the recursive helper
// for makeLexicalScopeTree
// It traverses the AST top-down,
// for every node-type that is present in `blockNodeTypes`,
// it :
//  1. Creates a new scope
//  2. Traverses the body of that block, and adds
//     all declarations in that block to the scope
//  3. For any sub-blocks, goes back to #1.
func makeLexicalScopeTree_(
	lang ParsedFile,
	node *sitter.Node,
	scope *Scope,
	scopeOfNode ScopeOfNode,
) *Scope {
	nodeType := node.Type()
	isBlockNode := slices.Contains(ScopeNodeTypes, nodeType)

	decls := lang.GetDecls(node)
	for _, decl := range decls {
		writeExpr, name := decl.InitExpr, decl.Name
		if writeExpr != nil && scope.Symbols[name] == nil {
			// add a new variable declaration to the scope
			// if it doesn't exist already
			scope.Symbols[name] = writeExpr
			scope.NameOfNode[writeExpr] = name
		}
	}

	nextScope := scope
	if isBlockNode {
		nextScope = &Scope{
			AstNode:          node,
			Parent:           scope,
			Symbols:          make(map[string]*sitter.Node),
			FilePathOfImport: make(map[*sitter.Node]string),
			NameOfNode:       make(map[*sitter.Node]string),
		}

		scopeOfNode[node] = nextScope
		if scope != nil {
			scope.Children = append(scope.Children, nextScope)
		} else {
			scope = nextScope // root
		}
	}

	for i := 0; i < int(node.NamedChildCount()); i++ {
		child := node.NamedChild(i)
		makeLexicalScopeTree_(lang, child, nextScope, scopeOfNode)
	}

	return scope
}

// findNearestBlockNode finds the nearest surrounding block node for any AST node
func findNearestBlockNode(node *sitter.Node) *sitter.Node {
	for node != nil {
		if slices.Contains(ScopeNodeTypes, node.Type()) {
			return node
		}
		node = node.Parent()
	}
	return nil
}

// GetScope finds the nearest surrounding scope of a node
func GetScope(module *Module, node *sitter.Node) *Scope {
	nearestBlock := findNearestBlockNode(node)
	if nearestBlock == nil {
		return nil
	}

	nearestScope, exists := module.ScopeOfNode[nearestBlock]
	if !exists {
		return nil
	}

	return nearestScope
}
