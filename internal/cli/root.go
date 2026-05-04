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

	// Version is overridden at build time via -ldflags.
	Version = "dev"
)

var rootCmd = &cobra.Command{
	Use:   "dotfiles",
	Short: "Manage your dotfiles by syncing local config with a checked-in mirror",
	Long: `dotfiles synchronizes a set of local configuration files with a mirror kept
inside a git repository. The list of managed paths is declared in a JSON
manifest. Files are copied (not symlinked) in either direction.`,
	SilenceUsage: true,
}

func init() {
	if Version == "dev" {
		if info, ok := debug.ReadBuildInfo(); ok && info.Main.Version != "" && info.Main.Version != "(devel)" {
			Version = info.Main.Version
		}
	}
	rootCmd.Version = Version

	rootCmd.PersistentFlags().StringVar(&rootPath, "root", "", "dotfiles repository root (default: $DOTFILES_ROOT or ~/.dotfiles)")
	rootCmd.PersistentFlags().StringVar(&manifestPath, "config", "", "manifest file path (default: <root>/dotfiles.json or $DOTFILES_CONFIG)")
	rootCmd.PersistentFlags().BoolVarP(&dryRun, "dry-run", "n", false, "preview actions without writing")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "log every file action")
	rootCmd.PersistentFlags().BoolVarP(&prune, "prune", "p", false, "remove destination files not in the manifest")
}

// Execute runs the CLI and exits the process with an appropriate status code.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			os.Exit(ee.ExitCode())
		}
		os.Exit(1)
	}
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
			return "", err
		}
		r = filepath.Join(home, ".dotfiles")
	}
	return filepath.Abs(r)
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
	return filepath.Abs(m)
}

// loadSpecs is a convenience for command implementations.
func loadSpecs() ([]dotfiles.Spec, error) {
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
		return nil, err
	}
	r := dotfiles.Resolver{RepoRoot: root, Home: home}
	return r.Resolve(m), nil
}
