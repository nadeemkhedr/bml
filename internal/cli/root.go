// Package cli wires bml's command surface (Cobra) to the act-on-a-URL core.
package cli

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"unicode/utf8"

	"bml/internal/browser"
	"bml/internal/config"
	"bml/internal/importer"
	"bml/internal/tui"

	"github.com/spf13/cobra"
)

// version is overridden at build time via -ldflags.
var version = "dev"

// BrowserFactory builds a Browser for the given macOS application name (empty =
// backend default). It's injected so tests can return a fake regardless of the
// configured browser.
type BrowserFactory func(app string) browser.Browser

// NewRootCmd builds the root command. The browser is constructed via mk from the
// configured application name once config is known.
func NewRootCmd(mk BrowserFactory) *cobra.Command {
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
				cfg, err := loadOrInit(cmd, configFlag)
				if err != nil {
					return err
				}
				return tui.RunLeader(mk(cfg.Browser), cfg.Bookmarks, cfg.Groups, cfg.LeaderTags, cfg.Search)
			}
			return resolveAndAct(cmd, mk, configFlag, args[0], newTab)
		},
	}

	cmd.Flags().BoolVarP(&newTab, "new-tab", "n", false, "force a new tab instead of focusing an existing one")
	cmd.PersistentFlags().StringVar(&configFlag, "config", "", "path to the config directory (default ~/.config/bml)")

	cmd.AddCommand(newEditCmd(&configFlag))
	cmd.AddCommand(newImportCmd(&configFlag))
	return cmd
}

// resolveAndAct turns a positional argument into a URL and acts on it.
//
//   - 1–3 characters, no "."  → a bookmark key sequence (errors if unbound)
//   - contains "."            → a URL (config not required)
//   - otherwise               → an error
func resolveAndAct(cmd *cobra.Command, mk BrowserFactory, configFlag, arg string, forceNew bool) error {
	if n := utf8.RuneCountInString(arg); n >= 1 && n <= 3 && !strings.Contains(arg, ".") {
		cfg, err := loadOrInit(cmd, configFlag)
		if err != nil {
			return err
		}
		url, ok := cfg.URLForKey(arg)
		if !ok {
			return fmt.Errorf("no bookmark bound to key %q", arg)
		}
		return mk(cfg.Browser).OpenOrFocus(url, forceNew)
	}
	if strings.Contains(arg, ".") {
		// A raw URL doesn't require a valid config; still honor the browser
		// setting if one is readable.
		dir, err := config.Dir(configFlag)
		if err != nil {
			return err
		}
		return mk(config.BrowserSetting(dir)).OpenOrFocus(arg, forceNew)
	}
	return fmt.Errorf("%q is neither a key (1–3 chars) nor a URL", arg)
}

// loadOrInit resolves the config directory, writing starter files on first run,
// then loads and validates it.
func loadOrInit(cmd *cobra.Command, configFlag string) (*config.Config, error) {
	dir, err := config.Dir(configFlag)
	if err != nil {
		return nil, err
	}
	created, err := config.WriteStarter(dir)
	if err != nil {
		return nil, fmt.Errorf("creating config: %w", err)
	}
	if created {
		fmt.Fprintf(cmd.ErrOrStderr(), "bml: created a starter config in %s — edit it with `bml edit`\n", dir)
	}
	return config.Load(dir)
}

// newEditCmd opens the bookmarks file in $EDITOR (creating starters on first
// run), or config.toml with --settings.
func newEditCmd(configFlag *string) *cobra.Command {
	var settings bool
	cmd := &cobra.Command{
		Use:           "edit",
		Short:         "open the bookmarks file in $EDITOR (--settings for config.toml)",
		Args:          cobra.NoArgs,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			dir, err := config.Dir(*configFlag)
			if err != nil {
				return err
			}
			if _, err := config.WriteStarter(dir); err != nil {
				return fmt.Errorf("creating config: %w", err)
			}
			path := config.BookmarksPath(dir)
			if settings {
				path = config.SettingsPath(dir)
			}
			return openEditor(cmd, path)
		},
	}
	cmd.Flags().BoolVar(&settings, "settings", false, "edit config.toml (settings) instead of bookmarks.toml")
	return cmd
}

// newImportCmd imports bookmarks from a Chromium browser into the config,
// merging by URL (preserving existing keyed favorites) unless --replace.
func newImportCmd(configFlag *string) *cobra.Command {
	var (
		profile string
		replace bool
		dryRun  bool
	)
	cmd := &cobra.Command{
		Use:   "import <browser>",
		Short: "import bookmarks from a Chromium browser",
		Long: "Import bookmarks from a Chromium-based browser (" +
			strings.Join(importer.SupportedNames(), ", ") + "). Folder names become " +
			"tags. By default new bookmarks are merged in (existing entries and their " +
			"keys are kept); use --replace to overwrite.",
		Args:          cobra.ExactArgs(1),
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			src, ok := importer.Lookup(args[0])
			if !ok {
				return fmt.Errorf("unknown browser %q; supported: %s", args[0], strings.Join(importer.SupportedNames(), ", "))
			}
			home, err := os.UserHomeDir()
			if err != nil {
				return err
			}
			imported, err := importer.Read(home, profile, src)
			if err != nil {
				return err
			}

			dir, err := config.Dir(*configFlag)
			if err != nil {
				return err
			}

			// Import only ever rewrites bookmarks.toml; settings in config.toml are
			// untouched. --replace starts from an empty list; the default merges
			// into the existing bookmarks, preserving keyed favorites.
			cfg := &config.Config{}
			if !replace {
				if existing, err := config.Load(dir); err == nil {
					cfg = existing
				} else if !os.IsNotExist(err) {
					return err // an existing-but-broken config shouldn't be silently clobbered
				}
			}

			added := cfg.Append(imported)

			if dryRun {
				text, err := config.RenderBookmarks(cfg)
				if err != nil {
					return err
				}
				fmt.Fprint(cmd.OutOrStdout(), text)
				fmt.Fprintf(cmd.ErrOrStderr(), "# dry run: %d new of %d from %s, %d total (not written)\n",
					added, len(imported), src.App, len(cfg.Bookmarks))
				return nil
			}

			backup, err := config.SaveBookmarks(dir, cfg)
			if err != nil {
				return err
			}
			if backup != "" {
				fmt.Fprintf(cmd.ErrOrStderr(), "backed up previous bookmarks to %s\n", backup)
			}
			fmt.Fprintf(cmd.ErrOrStderr(), "imported %d new bookmark(s) from %s — %d total → %s\n",
				added, src.App, len(cfg.Bookmarks), config.BookmarksPath(dir))
			return nil
		},
	}
	cmd.Flags().StringVar(&profile, "profile", "Default", "browser profile directory")
	cmd.Flags().BoolVar(&replace, "replace", false, "replace existing bookmarks instead of merging")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "print the resulting config to stdout without writing")
	return cmd
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
