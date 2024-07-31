package main

import (
	"fmt"
	"os"

	"github.com/srijanpaul-deepsource/reachable/pkg/sniper"
)

func test() {
	code := `

def main():
	print("hello world")

main()
`

	py, err := sniper.ParsePython("main.py", []byte(code))
	if err != nil {
		panic(err)
	}

	dg := sniper.DotGraphFromTsQuery(
		`(call function:(identifier) @id (.match? @id "main")) @call`,
		py,
	)

	if dg == nil {
		panic("dotgraph is nil")
	}

	fmt.Println(dg.String())
}

func main() {
	conf, err := ReadConfig()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	cli := NewCli(conf)
	err = cli.Run()
	if err != nil {
		panic(err)
	}
}
