package sniper

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func goProjectRoot() string {
	goProjectRoot, err := filepath.Abs("../../")
	if err != nil {
		panic(err)
	}

	return goProjectRoot
}

func readFile(absPath string) (string, error) {
	bytes, err := os.ReadFile(absPath)
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}

func Test_findProjectRoot(t *testing.T) {
	goProjectRoot := goProjectRoot()
	pyFilePath := filepath.Join(goProjectRoot, "test-projects/pyproject/src/mypackage/__main__.py")
	projectRoot, err := findProjectRoot(pyFilePath)
	require.NoError(t, err)
	require.NotNil(t, projectRoot)

	got, err := filepath.Rel(goProjectRoot, *projectRoot)
	require.NoError(t, err)
	want := "test-projects/pyproject"
	assert.Equal(t, want, got)
}

func Test_FilePathForImport(t *testing.T) {
	goProjectRoot := goProjectRoot()
	mainDotPy := filepath.Join(goProjectRoot, "test-projects/pyproject/src/mypackage/__main__.py")

	contents, err := readFile(mainDotPy)
	require.NoError(t, err)

	py, err := ParsePython(mainDotPy, contents)
	require.NoError(t, err)
	require.NotNil(t, py)

}
