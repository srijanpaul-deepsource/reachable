package main

import (
	"flag"
	"fmt"
	"os"

	sitter "github.com/smacker/go-tree-sitter"
	treeSitterPy "github.com/smacker/go-tree-sitter/python"
)

func getTsLanguage(langName string) (*sitter.Language, error) {
	switch langName {
	case "py", "python":
		return treeSitterPy.GetLanguage(), nil
	}

	return nil, fmt.Errorf("Language not supported: %s", langName)
}

type Config struct {
	Language    *sitter.Language
	ProjectRoot string
	Files       []string
}

func main() {
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
}
