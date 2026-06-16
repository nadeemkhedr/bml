package main

import (
	"errors"
	"fmt"
	"os"

	"bml/internal/browser"
	"bml/internal/cli"
)

func main() {
	mk := func(app string) browser.Browser { return browser.NewChromium(app) }

	if err := cli.NewRootCmd(mk).Execute(); err != nil {
		if errors.Is(err, browser.ErrAutomationDenied) {
			fmt.Fprintln(os.Stderr, "bml: macOS hasn't granted permission to control the browser.")
			fmt.Fprintln(os.Stderr, "     Grant it under System Settings → Privacy & Security → Automation,")
			fmt.Fprintln(os.Stderr, "     then run bml again.")
			os.Exit(1)
		}
		fmt.Fprintln(os.Stderr, "bml: "+err.Error())
		os.Exit(1)
	}
}
