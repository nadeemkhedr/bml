package main

import (
	"fmt"
	"os"

	"bml/internal/browser"
	"bml/internal/cli"
)

func main() {
	b := browser.NewChromium(browser.DefaultChromiumApp)
	if err := cli.NewRootCmd(b).Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "bml: "+err.Error())
		os.Exit(1)
	}
}
