package sniper

import (
	"fmt"
	"slices"
	"strings"

	"github.com/emicklei/dot"
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
	FilePathOfImport map[*sitter.Node]string
	AstNode          *sitter.Node
}

func (s *Scope) ToDotGraph() *dot.Graph {
	g := dot.NewGraph()
	s.toDotNode(g)
	return g
}

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
		FilePathOfImport: make(map[*sitter.Node]string),
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
			FilePathOfImport: make(map[*sitter.Node]string),
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
