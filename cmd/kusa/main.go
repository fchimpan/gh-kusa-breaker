package main

import (
	"os"

	"github.com/fchimpan/gh-kusa-breaker/cmd"
)

func main() {
	root := cmd.NewRootCmd(cmd.DefaultDeps())
	if err := root.Execute(); err != nil {
		os.Exit(1)
	}
}
