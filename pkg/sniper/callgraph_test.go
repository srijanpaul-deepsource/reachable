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
	py, err := ParsePython("test.py", []byte(code))
	require.NoError(t, err)
	require.NotNil(t, py)

	graph := DotGraphFromTsQuery(`(call function:(identifier) @id (.match? @id "baz")) @call`, py)
	require.NotNil(t, graph)
	assert.Len(t, graph.EdgesMap(), 2)

	want := removeWhitespace(`digraph {
		n1[label="test:baz"];
		n2[label="test:foo"];
		n3[label="test:f"];
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

	py, err := ParsePython("test.py", []byte(code))
	if err != nil {
		panic(err)
	}

	dg := DotGraphFromTsQuery(
		`(call function:(identifier) @id (.match? @id "f")) @call`,
		py,
	)

	want := removeWhitespace(`
		digraph {
			n1[label="test:f"];
			n2[label="test:g"];
			n3[label="test:bar"];
			n4[label="test:f2"];
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

func Test_CallGraphClassCtors(t *testing.T) {
	code := `
class A:
	def __init__(x: int) -> int:
		pass
	
def foo():
	A()

foo()
	`

	py, err := ParsePython("test.py", []byte(code))
	if err != nil {
		panic(err)
	}

	dg := DotGraphFromTsQuery(
		`(call function:(identifier) @id (.match? @id "foo")) @call`,
		py,
	)
	require.NotNil(t, dg)
	got := removeWhitespace(dg.String())

	// TODO(@Srijan/Tushar): ideally, this would be `A.__init__`, not `__init__`
	want := removeWhitespace(`digraph {
		n1[label="test:foo"];
		n2[label="test:__init__"];
		n1 -> n2;
	}`)

	assert.Equal(t, want, got)
}
