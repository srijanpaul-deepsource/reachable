package sniper

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var code = `
def f():
	return

def foo():
	f()

def baz():
	return foo()
baz()`

func removeWhitespace(s string) string {
	return strings.Join(strings.Fields(s), "")
}

func Test_CallGraph(t *testing.T) {
	py, err := ParsePython(code)
	require.NoError(t, err)
	require.NotNil(t, py)

	graph := DotGraphFromTsQuery(`(call function:(identifier) @id (.match? @id "baz")) @call`, py)
	require.NotNil(t, graph)
	assert.Len(t, graph.EdgesMap(), 2)

	want := removeWhitespace(`digraph {
		n1[label="baz"];
		n2[label="foo"];
		n3[label="f"];
		n1->n2;
		n2->n3;}`,
	)

	got := removeWhitespace(graph.String())

	assert.Equal(t, want, got)
}
