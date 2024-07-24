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

func Test_CallGraphRecursive(t *testing.T) {
	code := `
def f():
	g()
	x = f()
	def bar():
		return g()
	f2 = lambda x : x
	bar()
	return f2()

def g():
	f()
`

	py, err := ParsePython(code)
	if err != nil {
		panic(err)
	}

	dg := DotGraphFromTsQuery(
		`(call function:(identifier) @id (.match? @id "f")) @call`,
		py,
	)

	want := removeWhitespace(`
		digraph {
			n1[label="f"];
			n2[label="g"];
			n3[label="bar"];
			n4[label="f2"];
			n1->n2;
			n1->n1;
			n1->n3;
			n1->n4;
			n2->n1;
			n3->n2;
		}
	`)
	got := removeWhitespace(dg.String())

	assert.Equal(t, want, got)
}
