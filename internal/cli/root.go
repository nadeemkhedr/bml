// Package cli wires bml's command surface (Cobra) to the act-on-a-URL core.
package cli

import (
	"fmt"
	"strings"

	"bml/internal/browser"

	"github.com/spf13/cobra"
)

// version is overridden at build time via -ldflags.
var version = "dev"

// NewRootCmd builds the root command, acting through the given Browser. The
// Browser is injected so tests can pass a fake.
func NewRootCmd(b browser.Browser) *cobra.Command {
	var newTab bool

	cmd := &cobra.Command{
		Use:   "bml [url]",
		Short: "bookmark launcher — focus or open browser tabs",
		Long: "bml is a macOS terminal launcher for bookmarks. With no argument it " +
			"opens the interactive launcher; with a URL it focuses a matching tab or " +
			"opens a new one.",
		Args:          cobra.MaximumNArgs(1),
		Version:       version,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				// Leader mode lands here in Phase 3.
				return fmt.Errorf("interactive mode is not implemented yet; pass a URL, e.g. `bml github.com`")
			}
			return act(b, args[0], newTab)
		},
	}

	cmd.Flags().BoolVarP(&newTab, "new-tab", "n", false, "force a new tab instead of focusing an existing one")
	return cmd
}

// act resolves a positional argument and acts on it. Phase 1 handles URLs only
// (an argument containing "."); bookmark-key resolution arrives in Phase 2.
func act(b browser.Browser, arg string, forceNew bool) error {
	if strings.Contains(arg, ".") {
		return b.OpenOrFocus(arg, forceNew)
	}
	return fmt.Errorf("%q is not a URL; bookmark-key resolution is not implemented yet", arg)
}
