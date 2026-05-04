package cli

import (
	_ "embed"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/spf13/cobra"
)

// initAction is one step taken (or that would be taken) by `dotfiles init`.
//
// Action values:
//   - "create"     — file did not exist; written.
//   - "overwrite"  — file existed and was replaced (--force).
//   - "skip"       — file existed and was left untouched (no --force).
//   - "git-init"   — `git init` was run in the target directory.
//   - "git-skip"   — `.git` already present; nothing to do.
type initAction struct {
	Action  string `json:"action"`
	Path    string `json:"path"`
	Message string `json:"message,omitempty"`
}

type initResponse struct {
	Root    string       `json:"root"`
	DryRun  bool         `json:"dryRun"`
	Force   bool         `json:"force"`
	Actions []initAction `json:"actions"`
}

const initJSONShape = `JSON output shape:

  {
    "root":    "/abs/path",
    "dryRun":  bool,
    "force":   bool,
    "actions": [{"action": "create"|"overwrite"|"skip"|"git-init"|"git-skip", "path", "message"}, ...]
  }`

//go:embed templates/README.md
var initReadmeTemplate string

const initManifestTemplate = "{}\n"

var forceInit bool

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Scaffold a fresh dotfiles repository",
	Long: `Scaffold a fresh dotfiles repository.

Creates the target directory (resolved like every other command: --root, then
$DOTFILES_ROOT, then ~/.dotfiles), writes an empty dotfiles.json and a short
README.md aimed at the user of the dotfiles repository, and runs git init.

Refuses with an error if dotfiles.json or README.md already exists; pass
--force to overwrite both. An existing target directory without those files
is reused. An existing .git directory is left alone.

` + initJSONShape,
	Example: `  dotfiles init
  dotfiles init --root ~/my-dotfiles
  dotfiles init --force --dry-run`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runInit(cmd)
	},
}

func init() {
	initCmd.Flags().BoolVar(&forceInit, "force", false, "overwrite dotfiles.json and README.md if they exist")
	initCmd.Flags().BoolVarP(&dryRun, "dry-run", "n", false, "preview actions without writing")
	initCmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "log every file action (ignored when --json is set)")
	rootCmd.AddCommand(initCmd)
}

func runInit(cmd *cobra.Command) error {
	root, err := resolveRoot()
	if err != nil {
		return handleErr(cmd, err)
	}

	manifestFile := filepath.Join(root, "dotfiles.json")
	readmeFile := filepath.Join(root, "README.md")
	gitDir := filepath.Join(root, ".git")

	if !forceInit {
		if existsFile(manifestFile) {
			return handleErr(cmd, fmt.Errorf("%s already exists; pass --force to overwrite", manifestFile))
		}
		if existsFile(readmeFile) {
			return handleErr(cmd, fmt.Errorf("%s already exists; pass --force to overwrite", readmeFile))
		}
	}

	gitPath, gitErr := exec.LookPath("git")
	if gitErr != nil {
		return handleErr(cmd, fmt.Errorf("git not found on PATH: %w", gitErr))
	}

	out := cmd.OutOrStdout()
	logOut := io.Discard
	if verbose && !jsonOutput {
		logOut = out
	}

	actions := make([]initAction, 0, 3)

	if !dryRun {
		if err := os.MkdirAll(root, 0o755); err != nil {
			return handleErr(cmd, fmt.Errorf("create %s: %w", root, err))
		}
	}

	manifestAction, err := writeInitFile(manifestFile, initManifestTemplate, dryRun, forceInit, logOut)
	if err != nil {
		return handleErr(cmd, err)
	}
	actions = append(actions, manifestAction)

	readmeAction, err := writeInitFile(readmeFile, initReadmeTemplate, dryRun, forceInit, logOut)
	if err != nil {
		return handleErr(cmd, err)
	}
	actions = append(actions, readmeAction)

	gitAction, err := initGit(gitPath, root, gitDir, dryRun, logOut)
	if err != nil {
		return handleErr(cmd, err)
	}
	actions = append(actions, gitAction)

	if jsonOutput {
		return writeJSON(out, initResponse{
			Root:    root,
			DryRun:  dryRun,
			Force:   forceInit,
			Actions: actions,
		})
	}

	if dryRun {
		fmt.Fprintf(out, "Initialized %s [DRY RUN]\n", root)
	} else {
		fmt.Fprintf(out, "Initialized %s\n", root)
	}
	return nil
}

func writeInitFile(path, content string, dryRun, force bool, logOut io.Writer) (initAction, error) {
	exists := existsFile(path)
	if exists && !force {
		fmt.Fprintf(logOut, "skip %s\n", path)
		return initAction{Action: "skip", Path: path, Message: "already exists"}, nil
	}
	action := "create"
	if exists {
		action = "overwrite"
	}
	if !dryRun {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return initAction{}, fmt.Errorf("create %s: %w", filepath.Dir(path), err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			return initAction{}, fmt.Errorf("write %s: %w", path, err)
		}
	}
	fmt.Fprintf(logOut, "%s %s\n", action, path)
	return initAction{Action: action, Path: path}, nil
}

func initGit(gitPath, root, gitDir string, dryRun bool, logOut io.Writer) (initAction, error) {
	if st, err := os.Stat(gitDir); err == nil && st.IsDir() {
		fmt.Fprintf(logOut, "git already initialized in %s\n", root)
		return initAction{Action: "git-skip", Path: gitDir, Message: "already initialized"}, nil
	}
	if dryRun {
		fmt.Fprintf(logOut, "git init %s\n", root)
		return initAction{Action: "git-init", Path: gitDir}, nil
	}
	cmd := exec.Command(gitPath, "init", root)
	cmd.Stdout = logOut
	cmd.Stderr = logOut
	if err := cmd.Run(); err != nil {
		return initAction{}, fmt.Errorf("git init %s: %w", root, err)
	}
	return initAction{Action: "git-init", Path: gitDir}, nil
}

func existsFile(p string) bool {
	st, err := os.Stat(p)
	return err == nil && !st.IsDir()
}
