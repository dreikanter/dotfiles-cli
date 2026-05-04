package cli

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime/debug"

	"github.com/dreikanter/dotfiles-cli/internal/dotfiles"
	"github.com/spf13/cobra"
)

var (
	rootPath     string
	manifestPath string
	dryRun       bool
	verbose      bool
	prune        bool
	jsonOutput   bool
	filterTool   string
	filterFiles  []string

	// Version is overridden at build time via -ldflags.
	Version = "dev"
)

var rootCmd = &cobra.Command{
	Use:   "dotfiles",
	Short: "Manage your dotfiles by syncing local config with a checked-in mirror",
	Long: `Manage your dotfiles by syncing local config with a checked-in mirror.

Tracks a set of local configuration files declared in a JSON manifest and
copies (not symlinks) them between the live filesystem and a git-managed
mirror.`,
	SilenceUsage:  true,
	SilenceErrors: true,
}

func init() {
	if Version == "" || Version == "dev" {
		if info, ok := debug.ReadBuildInfo(); ok && info.Main.Version != "" && info.Main.Version != "(devel)" {
			Version = info.Main.Version
		}
	}
	if Version == "" {
		Version = "unknown"
	}
	rootCmd.Version = Version

	rootCmd.PersistentFlags().StringVar(&rootPath, "root", "", "dotfiles repository root (default: $DOTFILES_ROOT or ~/.dotfiles)")
	rootCmd.PersistentFlags().StringVar(&manifestPath, "config", "", "manifest file path (default: <root>/dotfiles.json or $DOTFILES_CONFIG)")
	rootCmd.PersistentFlags().BoolVar(&jsonOutput, "json", false, "emit a single JSON object on stdout (non-zero exit on failure)")
}

// Execute runs the CLI and exits the process with an appropriate status code.
func Execute() {
	err := rootCmd.Execute()
	if err == nil {
		return
	}
	if errors.Is(err, errSilent) {
		os.Exit(1)
	}
	var ee *exec.ExitError
	if errors.As(err, &ee) {
		os.Exit(ee.ExitCode())
	}
	if jsonOutput {
		_ = writeJSON(rootCmd.OutOrStdout(), errorEnvelope{Error: errorBody{Message: err.Error()}})
	} else {
		fmt.Fprintln(rootCmd.ErrOrStderr(), "Error:", err)
	}
	os.Exit(1)
}

// resolveRoot returns the absolute repository root path.
func resolveRoot() (string, error) {
	r := rootPath
	if r == "" {
		r = os.Getenv("DOTFILES_ROOT")
	}
	if r == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("resolve home dir: %w", err)
		}
		r = filepath.Join(home, ".dotfiles")
	}
	abs, err := filepath.Abs(r)
	if err != nil {
		return "", fmt.Errorf("resolve absolute path of %s: %w", r, err)
	}
	return abs, nil
}

// resolveManifest returns the absolute manifest path.
func resolveManifest(root string) (string, error) {
	m := manifestPath
	if m == "" {
		m = os.Getenv("DOTFILES_CONFIG")
	}
	if m == "" {
		m = filepath.Join(root, "dotfiles.json")
	}
	abs, err := filepath.Abs(m)
	if err != nil {
		return "", fmt.Errorf("resolve absolute path of %s: %w", m, err)
	}
	return abs, nil
}

// loadSpecs resolves the manifest and filters specs through sel. Validation
// runs before any IO, so a bad filter (unknown tool, file not declared) is a
// hard error with no side effects.
func loadSpecs(sel dotfiles.Selector) ([]dotfiles.Spec, error) {
	root, err := resolveRoot()
	if err != nil {
		return nil, err
	}
	mPath, err := resolveManifest(root)
	if err != nil {
		return nil, err
	}
	m, err := dotfiles.LoadManifest(mPath)
	if err != nil {
		return nil, fmt.Errorf("load manifest: %w", err)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("resolve home dir: %w", err)
	}
	r := dotfiles.Resolver{RepoRoot: root, Home: home}
	return sel.Apply(r.Resolve(m), home)
}

// currentSelector reads the package-level filter flags into a Selector.
func currentSelector() dotfiles.Selector {
	return dotfiles.Selector{Tool: filterTool, Files: filterFiles}
}

// addFilterFlags registers --tool and --file on cmd and binds them to the
// package-level filter variables.
func addFilterFlags(cmd *cobra.Command) {
	cmd.Flags().StringVar(&filterTool, "tool", "", "restrict to a single manifest tool")
	cmd.Flags().StringArrayVar(&filterFiles, "file", nil,
		"restrict to specific files within --tool (repeatable; expanded as ~ and CWD-relative; matched literally against manifest entries)")
}
