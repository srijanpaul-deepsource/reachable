package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/fatih/color"
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
	ShowDotGraph bool
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
	showDotGraph := flag.Bool("dotgraph", false, "Show the call graph in dot format")

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
		ShowDotGraph: *showDotGraph,
	}

	return config, nil
}

type Cli struct {
	// language sitter.Language
	files        []string
	lockFilePath string
	moduleCache  map[string]sniper.ParsedFile
	showDotGraph bool
}

func NewCli(conf *Config) *Cli {
	return &Cli{
		files:        conf.Files,
		moduleCache:  make(map[string]sniper.ParsedFile),
		lockFilePath: conf.LockfilePath,
		showDotGraph: conf.ShowDotGraph,
	}
}

type VulnDep struct {
	packageName string
	osvVulnId   string
	osvVulnDesc string
}

func collectVulnerableDepNames(report models.VulnerabilityResults) map[string]VulnDep {
	depNames := make(map[string]VulnDep)
	for _, result := range report.Results {
		for _, pkg := range result.Packages {
			if len(pkg.Vulnerabilities) > 0 {
				depNames[pkg.Package.Name] = VulnDep{
					packageName: pkg.Package.Name,
					osvVulnId:   pkg.Vulnerabilities[0].ID,
					osvVulnDesc: pkg.Vulnerabilities[0].Summary,
				}
			}
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

	yellow := color.New(color.FgYellow).SprintFunc()
	bgRed := color.New(color.FgRed).Add(color.Bold).SprintFunc()
	green := color.New(color.FgGreen).Add(color.Bold).SprintFunc()
	grey := color.New(color.FgHiBlue).Add(color.Bold).SprintFunc()

	visitCallGraphNode := func(cgNode *sniper.CgNode, path []*sniper.CgNode) {
		packageName := cgNode.File.PackageName()
		if packageName == nil {
			return
		}

		// TODO: Should we early exit
		if _, exists := vulnPackages[*packageName]; exists {
			if cgNode.FuncName != nil {
				fmt.Printf("%s: Vulnerabily found in dependency %s\n", bgRed("ALERT"), yellow(*packageName))

				fmt.Println("Stack trace:")
				for i, node := range path {
					if i == 0 {
						continue
					}

					if node.FuncName != nil {
						filePath, _ := filepath.Rel(*node.File.Module().ProjectRoot, node.File.Module().FileName)
						prefix := "which calls "

						if i == 1 {
							prefix = "in function "
						}

						suffix := ""
						packageName := node.File.PackageName()
						if packageName != nil {
							suffix = fmt.Sprintf(" (package %s) ", grey(*packageName))
						}

						fmt.Printf(
							"    %s%s in %s%s\n", prefix,
							yellow(*node.FuncName), filePath, suffix,
						)
					}
				}

				fmt.Print("\nVulnerability details:\n")
				fmt.Printf("%s: %s\n", green("ID"), vulnPackages[*packageName].osvVulnId)
				fmt.Printf("%s: %s\n", green("Description"), vulnPackages[*packageName].osvVulnDesc)

				fmt.Print("\n\n")
				delete(vulnPackages, *packageName)
			}
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

		if c.showDotGraph {
			dotGraph := sniper.Cg2Dg(callGraph)
			fmt.Println(dotGraph.String())
		} else {
			callGraph.Walk(c.files[0], visitCallGraphNode)
		}

	}

	return nil
}
