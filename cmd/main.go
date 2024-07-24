package main

import (
	"flag"
	"fmt"
	"os"

	sitter "github.com/smacker/go-tree-sitter"
	"github.com/srijanpaul-deepsource/reachable/pkg/sniper"

	treeSitterPy "github.com/smacker/go-tree-sitter/python"
)

func getTsLanguage(langName string) (*sitter.Language, error) {
	switch langName {
	case "py", "python":
		return treeSitterPy.GetLanguage(), nil
	}

	return nil, fmt.Errorf("language not supported: %s", langName)
}

type Config struct {
	Language    *sitter.Language
	ProjectRoot string
	Files       []string
}

func test() {
	code := `
def f():
	return

def foo():
	f()

def baz():
	return foo()


baz()
`

	py, err := sniper.ParsePython(code)
	if err != nil {
		panic(err)
	}

	dg := sniper.DotGraphFromTsQuery(`(call function:(identifier) @id (.match? @id "baz")) @call`, py)
	fmt.Println(dg.String())
}

func main() {
	test()
	os.Exit(0)

	// Define flags
	repoRoot := flag.String("repo-root", "", "Root directory of the repository")
	language := flag.String("language", "", "Programming language to be used")

	// Parse the flags
	flag.Parse()

	// Get positional arguments (the files)
	files := flag.Args()

	// Check if required flags are set
	if *repoRoot == "" {
		fmt.Fprint(os.Stderr, "Error: --repo-root is required")
		os.Exit(1)
	}
	if *language == "" {
		fmt.Fprint(os.Stderr, "Error: --language is required")
		os.Exit(1)
	}

	tsLanguage, err := getTsLanguage(*language)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed: %s\n", err.Error())
		os.Exit(1)
	}

	config := Config{
		Language:    tsLanguage,
		ProjectRoot: *repoRoot,
		Files:       files,
	}

	fmt.Sprintf("%#v", config)

}
