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
`

var pyBytes = []byte(pySource)

func Test_(t *testing.T) {
	py, err := ParsePython(pySource)
	require.NoError(t, err)
	require.NotNil(t, py)

	scope, scopeOfNode := py.module.GlobalScope, py.module.ScopeOfNode
	require.NotNil(t, scope)
	require.NotNil(t, scopeOfNode)

	require.Empty(t, scope.Symbols)
	require.Len(t, scope.Children, 1)
	child := scope.Children[0]
	require.NotNil(t, child)

	assert.Contains(t, child.Symbols, "foo")
	assert.Contains(t, child.Symbols, "x")
	assert.Equal(t, "'x'", child.Symbols["x"].Content(pyBytes))

	// test assignment patterns: y, z = 1, 2
	assert.Contains(t, child.Symbols, "y")
	assert.Contains(t, child.Symbols, "z")

	// type annotations a, b: Tuple[int, int] = 1, 2
	assert.Contains(t, child.Symbols, "a")
	assert.Equal(t, "1", child.Symbols["a"].Content(pyBytes))
	assert.Contains(t, child.Symbols, "b")
	assert.Equal(t, "2", child.Symbols["b"].Content(pyBytes))

	assert.NotContains(t, child.Symbols, "bar")

	require.Len(t, child.Children, 1)
	child = child.Children[0]
	require.NotNil(t, child)
	require.Contains(t, child.Symbols, "bar")

	require.Len(t, child.Children, 1)
	child = child.Children[0]
	require.NotNil(t, child)
	require.Contains(t, child.Symbols, "baz")
	assert.Equal(t, "420", child.Symbols["baz"].Content(pyBytes))
}
