package cli

import (
	"fmt"
	"os"

	"github.com/dreikanter/dotfiles-cli/internal/dotfiles"
	"github.com/spf13/cobra"
)

var (
	trackTool    string
	untrackTool  string
	untrackPurge bool
)

type trackResponse struct {
	Action string `json:"action"`
	Tool   string `json:"tool"`
	Path   string `json:"path"`
}

type untrackResponse struct {
	Action string `json:"action"`
	Tool   string `json:"tool"`
	Path   string `json:"path"`
	Purged string `json:"purged,omitempty"`
}

var trackCmd = &cobra.Command{
	Use:   "track <path>",
	Short: "Start tracking a config file or directory",
	Long: `Start tracking a config file or directory under the given tool name.

The path must exist on disk. Directories are recognized automatically and
their contents tracked recursively. Run ` + "`dotfiles save`" + ` afterwards
to copy the file into the repository for the first time.`,
	Args: cobra.ExactArgs(1),
	RunE: runTrack,
}

var untrackCmd = &cobra.Command{
	Use:   "untrack <path>",
	Short: "Stop tracking a config file or directory",
	Long: `Stop tracking a path that was previously added with ` + "`dotfiles track`" + `.

The live file is left untouched. Pass --purge to also remove the saved
copy from the repository.`,
	Args: cobra.ExactArgs(1),
	RunE: runUntrack,
}

func init() {
	trackCmd.Flags().StringVar(&trackTool, "tool", "", "tool name to add the path under (required)")
	_ = trackCmd.MarkFlagRequired("tool")
	trackCmd.Flags().BoolVarP(&dryRun, "dry-run", "n", false, "show what would change without writing")
	rootCmd.AddCommand(trackCmd)

	untrackCmd.Flags().StringVar(&untrackTool, "tool", "", "tool name to remove the path from (required)")
	_ = untrackCmd.MarkFlagRequired("tool")
	untrackCmd.Flags().BoolVar(&untrackPurge, "purge", false, "also delete the saved copy from the repository")
	untrackCmd.Flags().BoolVarP(&dryRun, "dry-run", "n", false, "show what would change without writing")
	rootCmd.AddCommand(untrackCmd)
}

func runTrack(cmd *cobra.Command, args []string) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return handleErr(cmd, fmt.Errorf("resolve home dir: %w", err))
	}

	abs := dotfiles.Expand(args[0], home)
	info, err := os.Stat(abs)
	if err != nil {
		if os.IsNotExist(err) {
			return handleErr(cmd, fmt.Errorf("%s: path does not exist", abs))
		}
		return handleErr(cmd, fmt.Errorf("stat %s: %w", abs, err))
	}

	mpath := dotfiles.ManifestPath(abs, home)
	if info.IsDir() {
		mpath += "/"
	}

	root, err := resolveRoot()
	if err != nil {
		return handleErr(cmd, err)
	}
	mfile, err := resolveManifest(root)
	if err != nil {
		return handleErr(cmd, err)
	}
	m, err := dotfiles.LoadManifest(mfile)
	if err != nil {
		return handleErr(cmd, err)
	}

	// check if already tracked under any tool
	for tool, paths := range m {
		for _, p := range paths {
			if dotfiles.Expand(p, home) == abs {
				return writeTrackResult(cmd, trackResponse{
					Action: "already-tracked",
					Tool:   tool,
					Path:   p,
				})
			}
		}
	}

	m[trackTool] = append(m[trackTool], mpath)
	if !dryRun {
		if err := dotfiles.SaveManifest(mfile, m); err != nil {
			return handleErr(cmd, err)
		}
	}

	return writeTrackResult(cmd, trackResponse{
		Action: "tracked",
		Tool:   trackTool,
		Path:   mpath,
	})
}

func runUntrack(cmd *cobra.Command, args []string) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return handleErr(cmd, fmt.Errorf("resolve home dir: %w", err))
	}

	abs := dotfiles.Expand(args[0], home)

	root, err := resolveRoot()
	if err != nil {
		return handleErr(cmd, err)
	}
	mfile, err := resolveManifest(root)
	if err != nil {
		return handleErr(cmd, err)
	}
	m, err := dotfiles.LoadManifest(mfile)
	if err != nil {
		return handleErr(cmd, err)
	}

	paths, toolExists := m[untrackTool]
	if !toolExists {
		if hint := findTrackingTool(m, abs, home); hint != "" {
			return handleErr(cmd, fmt.Errorf("%s is tracked under tool %q, not %q", abs, hint, untrackTool))
		}
		return handleErr(cmd, fmt.Errorf("tool %q not found in manifest", untrackTool))
	}

	foundIdx := -1
	var foundPath string
	for i, p := range paths {
		if dotfiles.Expand(p, home) == abs {
			foundIdx = i
			foundPath = p
			break
		}
	}
	if foundIdx < 0 {
		if hint := findTrackingTool(m, abs, home); hint != "" {
			return handleErr(cmd, fmt.Errorf("%s is tracked under tool %q, not %q", abs, hint, untrackTool))
		}
		return handleErr(cmd, fmt.Errorf("%s not found under tool %q", abs, untrackTool))
	}

	// resolve saved path before modifying the manifest
	var savedPath string
	if untrackPurge {
		r := dotfiles.Resolver{RepoRoot: root, Home: home}
		for _, s := range r.Resolve(m) {
			if s.Tool == untrackTool && s.LivePath == abs {
				savedPath = s.SavedPath
				break
			}
		}
	}

	newPaths := make([]string, 0, len(paths)-1)
	newPaths = append(newPaths, paths[:foundIdx]...)
	newPaths = append(newPaths, paths[foundIdx+1:]...)
	if len(newPaths) == 0 {
		delete(m, untrackTool)
	} else {
		m[untrackTool] = newPaths
	}

	var purged string
	if !dryRun {
		if err := dotfiles.SaveManifest(mfile, m); err != nil {
			return handleErr(cmd, err)
		}
		if untrackPurge && savedPath != "" {
			if err := os.RemoveAll(savedPath); err != nil {
				return handleErr(cmd, fmt.Errorf("purge %s: %w", savedPath, err))
			}
			purged = savedPath
		}
	} else if untrackPurge && savedPath != "" {
		purged = savedPath
	}

	return writeUntrackResult(cmd, untrackResponse{
		Action: "untracked",
		Tool:   untrackTool,
		Path:   foundPath,
		Purged: purged,
	})
}

// findTrackingTool returns the tool name that tracks abs (if any), for error hints.
func findTrackingTool(m dotfiles.Manifest, abs, home string) string {
	for tool, paths := range m {
		for _, p := range paths {
			if dotfiles.Expand(p, home) == abs {
				return tool
			}
		}
	}
	return ""
}

func writeTrackResult(cmd *cobra.Command, resp trackResponse) error {
	out := cmd.OutOrStdout()
	if jsonOutput {
		return writeJSON(out, resp)
	}
	fmt.Fprintf(out, "%s %s %s\n", resp.Action, resp.Tool, resp.Path)
	return nil
}

func writeUntrackResult(cmd *cobra.Command, resp untrackResponse) error {
	out := cmd.OutOrStdout()
	if jsonOutput {
		return writeJSON(out, resp)
	}
	fmt.Fprintf(out, "%s %s %s\n", resp.Action, resp.Tool, resp.Path)
	if resp.Purged != "" {
		fmt.Fprintf(out, "purged %s\n", resp.Purged)
	}
	return nil
}
