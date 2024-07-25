package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	sitter "github.com/smacker/go-tree-sitter"
	treeSitterPy "github.com/smacker/go-tree-sitter/python"
	"github.com/srijanpaul-deepsource/reachable/pkg/sniper"
)

type Config struct {
	Language    *sitter.Language
	ProjectRoot *string
	Files       []string
}

func getTsLanguage(langName string) (*sitter.Language, error) {
	switch langName {
	case "py", "python":
		return treeSitterPy.GetLanguage(), nil
	}

	return nil, fmt.Errorf("language not supported: %s", langName)
}

func ReadConfig() (*Config, error) {
	repoRoot := flag.String("repo-root", "", "Root directory of the repository")
	language := flag.String("language", "", "Programming language to be used")

	if *repoRoot == "" {
		repoRoot = nil
	}

	flag.Parse()
	files := flag.Args() // read positional args

	if *language == "" {
		fmt.Fprint(os.Stderr, "Error: --language is required")
		os.Exit(1)
	}

	tsLanguage, err := getTsLanguage(*language)
	if err != nil {
		return nil, fmt.Errorf("Failed: %s\n", err.Error())
	}

	config := &Config{
		Language:    tsLanguage,
		ProjectRoot: repoRoot,
		Files:       files,
	}

	return config, nil
}

type Cli struct {
	// language sitter.Language
	files       []string
	moduleCache map[string]sniper.ParsedFile
}

func NewCli(conf *Config) *Cli {
	return &Cli{
		files:       conf.Files,
		moduleCache: make(map[string]sniper.ParsedFile),
	}
}

func (c *Cli) Run() error {
	for _, file := range c.files {
		file, err := filepath.Abs(file)
		if err != nil {
			return err
		}

		fileContent, err := os.ReadFile(file)
		if err != nil {
			return err
		}

		py, err := sniper.ParsePython(file, fileContent)
		if err != nil {
			return err
		}

		dg := sniper.DotGraphFromFile(py, c.moduleCache)
		fmt.Println(dg.String())
	}

	return nil
}
