package sniper

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const pySource = `
x = 'x'
def foo():
		def bar():
			baz = 420
			return 1	
		return bar()	
		
y, z = 1, 2
a, b: Tuple[int, int] = 1, 2

class Foo:
	def __init__(self: Foo):
		pass
`

var pyBytes = []byte(pySource)

func Test_Scope(t *testing.T) {
	py, err := ParsePython("test.py", pyBytes)
	require.NoError(t, err)
	require.NotNil(t, py)

	scope, scopeOfNode := py.module.GlobalScope, py.module.ScopeOfNode
	require.NotNil(t, scope)
	require.NotNil(t, scopeOfNode)

	assert.Contains(t, scope.Symbols, "foo")
	assert.Contains(t, scope.Symbols, "x")
	assert.Equal(t, "'x'", scope.Symbols["x"].Content(pyBytes))

	// test class decl: class Foo:
	assert.Contains(t, scope.Symbols, "Foo")
	assert.Equal(t, "class_definition", scope.Symbols["Foo"].Type())

	// test assignment patterns: y, z = 1, 2
	assert.Contains(t, scope.Symbols, "y")
	assert.Contains(t, scope.Symbols, "z")

	// type annotations a, b: Tuple[int, int] = 1, 2
	assert.Contains(t, scope.Symbols, "a")
	assert.Equal(t, "1", scope.Symbols["a"].Content(pyBytes))
	assert.Contains(t, scope.Symbols, "b")
	assert.Equal(t, "2", scope.Symbols["b"].Content(pyBytes))

	assert.NotContains(t, scope.Symbols, "bar")

	require.Len(t, scope.Children, 2)
	child := scope.Children[0]
	require.NotNil(t, child)
	require.Contains(t, child.Symbols, "bar")

	require.Len(t, child.Children, 1)
	child = child.Children[0]
	require.NotNil(t, child)
	require.Contains(t, child.Symbols, "baz")
	assert.Equal(t, "420", child.Symbols["baz"].Content(pyBytes))
}
