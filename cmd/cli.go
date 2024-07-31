package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/google/osv-scanner/pkg/models"
	osv "github.com/google/osv-scanner/pkg/osvscanner"
	sitter "github.com/smacker/go-tree-sitter"
	treeSitterPy "github.com/smacker/go-tree-sitter/python"
	"github.com/srijanpaul-deepsource/reachable/pkg/sniper"
)

type Config struct {
	Language     *sitter.Language
	ProjectRoot  *string
	LockfilePath string
	Files        []string
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
	lockFilePath := flag.String("lockfile", "", "Path to the lockfile")

	flag.Parse()
	files := flag.Args() // read positional args

	if *repoRoot == "" {
		repoRoot = nil
	}

	if *language == "" {
		return nil, fmt.Errorf("error: --language is required")
	}

	if *lockFilePath == "" {
		return nil, fmt.Errorf("error: --lockfile is required")
	}

	tsLanguage, err := getTsLanguage(*language)
	if err != nil {
		return nil, fmt.Errorf("failed: %s", err.Error())
	}

	config := &Config{
		Language:     tsLanguage,
		ProjectRoot:  repoRoot,
		LockfilePath: *lockFilePath,
		Files:        files,
	}

	return config, nil
}

type Cli struct {
	// language sitter.Language
	files        []string
	lockFilePath string
	moduleCache  map[string]sniper.ParsedFile
}

func NewCli(conf *Config) *Cli {
	return &Cli{
		files:        conf.Files,
		moduleCache:  make(map[string]sniper.ParsedFile),
		lockFilePath: conf.LockfilePath,
	}
}

func collectVulnerableDepNames(report models.VulnerabilityResults) map[string]struct{} {
	depNames := make(map[string]struct{})
	for _, result := range report.Results {
		for _, pkg := range result.Packages {
			depNames[pkg.Package.Name] = struct{}{}
		}
	}

	return depNames
}

func (c *Cli) Run() error {
	// step 1: Run OSV Scanner to find out vulnerable dependencies
	scannerConfig := osv.ScannerActions{
		LockfilePaths: []string{c.lockFilePath},
	}

	result, err := osv.DoScan(scannerConfig, nil)
	if err != nil && !errors.Is(err, osv.VulnerabilitiesFoundErr) {
		return err
	}

	vulnPackages := collectVulnerableDepNames(result)
	// var vulnPackages = make(map[string]struct{})

	visitCallGraphNode := func(cgNode *sniper.CgNode) {
		packageName := cgNode.File.PackageName()
		if packageName == nil {
			return
		}

		// TODO: Should we early exit
		if _, exists := vulnPackages[*packageName]; exists {
			fName := "[unknown function]"
			if cgNode.FuncName != nil {
				fName = *cgNode.FuncName
			}

			fmt.Fprintf(os.Stderr, "pwned via %s because of call to %s\n", *packageName, fName)
		}
	}

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

		callGraph := sniper.CallGraphFromFile(py, c.moduleCache)
		callGraph.Walk(visitCallGraphNode)

		dotGraph := sniper.Cg2Dg(callGraph)
		fmt.Println(dotGraph.String())
	}

	return nil
}
