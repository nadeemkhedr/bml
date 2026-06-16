// Package cli wires bml's command surface (Cobra) to the act-on-a-URL core.
package cli

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"unicode/utf8"

	"bml/internal/browser"
	"bml/internal/config"

	"github.com/spf13/cobra"
)

// version is overridden at build time via -ldflags.
var version = "dev"

// NewRootCmd builds the root command, acting through the given Browser. The
// Browser is injected so tests can pass a fake.
func NewRootCmd(b browser.Browser) *cobra.Command {
	var (
		newTab     bool
		configFlag string
	)

	cmd := &cobra.Command{
		Use:   "bml [url|key]",
		Short: "bookmark launcher — focus or open browser tabs",
		Long: "bml is a macOS terminal launcher for bookmarks. With no argument it " +
			"opens the interactive launcher; with a single-character key it acts on " +
			"that bookmark; with a URL it acts on the URL.",
		Args:          cobra.MaximumNArgs(1),
		Version:       version,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				// Leader mode lands here in Phase 3.
				return errors.New("interactive mode is not implemented yet; pass a URL or key, e.g. `bml github.com` or `bml g`")
			}
			return resolveAndAct(cmd, b, configFlag, args[0], newTab)
		},
	}

	cmd.Flags().BoolVarP(&newTab, "new-tab", "n", false, "force a new tab instead of focusing an existing one")
	cmd.PersistentFlags().StringVar(&configFlag, "config", "", "path to the bookmark file (default ~/.config/bml/bookmarks.toml)")

	cmd.AddCommand(newEditCmd(&configFlag))
	return cmd
}

// resolveAndAct turns a positional argument into a URL and acts on it.
//
//   - exactly one character  → a bookmark key (errors if unbound)
//   - contains "."           → a URL (config not required)
//   - otherwise              → an error
func resolveAndAct(cmd *cobra.Command, b browser.Browser, configFlag, arg string, forceNew bool) error {
	if utf8.RuneCountInString(arg) == 1 {
		cfg, err := loadOrInit(cmd, configFlag)
		if err != nil {
			return err
		}
		url, ok := cfg.URLForKey(arg)
		if !ok {
			return fmt.Errorf("no bookmark bound to key %q", arg)
		}
		return b.OpenOrFocus(url, forceNew)
	}
	if strings.Contains(arg, ".") {
		return b.OpenOrFocus(arg, forceNew)
	}
	return fmt.Errorf("%q is neither a single-character key nor a URL", arg)
}

// loadOrInit resolves the config path, writing a starter file on first run, then
// loads and validates it.
func loadOrInit(cmd *cobra.Command, configFlag string) (*config.Config, error) {
	path, err := config.Path(configFlag)
	if err != nil {
		return nil, err
	}
	created, err := config.WriteStarter(path)
	if err != nil {
		return nil, fmt.Errorf("creating config: %w", err)
	}
	if created {
		fmt.Fprintf(cmd.ErrOrStderr(), "bml: created a starter config at %s — edit it with `bml edit`\n", path)
	}
	return config.Load(path)
}

// newEditCmd opens the bookmark file in $EDITOR (creating a starter first run).
func newEditCmd(configFlag *string) *cobra.Command {
	return &cobra.Command{
		Use:           "edit",
		Short:         "open the bookmark file in $EDITOR",
		Args:          cobra.NoArgs,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			path, err := config.Path(*configFlag)
			if err != nil {
				return err
			}
			if _, err := config.WriteStarter(path); err != nil {
				return fmt.Errorf("creating config: %w", err)
			}
			return openEditor(cmd, path)
		},
	}
}

func openEditor(cmd *cobra.Command, path string) error {
	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = "vi"
	}
	ed := exec.Command(editor, path)
	ed.Stdin, ed.Stdout, ed.Stderr = stdin(cmd), cmd.OutOrStdout(), cmd.ErrOrStderr()
	return ed.Run()
}

// stdin returns the command's input stream, defaulting to os.Stdin.
func stdin(cmd *cobra.Command) io.Reader {
	if in := cmd.InOrStdin(); in != nil {
		return in
	}
	return os.Stdin
}
