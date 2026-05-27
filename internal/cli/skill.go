package cli

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

const (
	skillName        = "dotfiles"
	skillDescription = "Use when working with the user's dotfiles managed by the `dotfiles` CLI — saving live config into the tracked repository, installing tracked files to live paths, inspecting tracked-file status, or scaffolding a new dotfiles repo."
)

const skillJSONShape = `JSON output shape (stdout mode):

  {
    "name":        "dotfiles",
    "description": "Use when ...",
    "body":        "...markdown body..."
  }

JSON output shape (install mode, --install):

  {
    "actions": [
      {"agent": "claude", "path": "/abs/path/SKILL.md",
       "action": "create"|"overwrite"|"skip"|"conflict",
       "error": "..." (omitted when empty)}, ...
    ]
  }`

// Skill is the rendered agent skill document for the current dotfiles binary.
type Skill struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Body        string `json:"body"`
}

// Markdown returns the full skill document including YAML frontmatter.
func (s Skill) Markdown() string {
	return "---\nname: " + s.Name + "\ndescription: " + s.Description + "\n---\n\n" + s.Body
}

// agentTarget describes a supported AI agent and how the skill is installed
// into it.
type agentTarget struct {
	// Name is the value accepted by --agent.
	Name string
	// PathFor returns the absolute install path for this agent.
	PathFor func() (string, error)
	// Detect returns true if the agent appears installed on this machine
	// (i.e. the containing skills directory exists).
	Detect func() (bool, error)
}

// installAction is one planned (or executed) filesystem step for one agent.
//
// Action values:
//   - "create"    — no file existed; will be / was written.
//   - "overwrite" — file existed with different content; --force was set.
//   - "skip"      — file existed with byte-identical content; no-op.
//   - "conflict"  — file existed with different content; --force was NOT set.
type installAction struct {
	Agent  string `json:"agent"`
	Path   string `json:"path"`
	Action string `json:"action,omitempty"`
	Error  string `json:"error,omitempty"`
}

type installResponse struct {
	Actions []installAction `json:"actions"`
}

// agents is the registry of supported install targets. Add a new entry to
// support a new agent; iteration order is preserved.
var agents = []agentTarget{
	{
		Name:    "claude",
		PathFor: claudeSkillPath,
		Detect:  claudeDetect,
	},
}

func claudeSkillPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home dir: %w", err)
	}
	return filepath.Join(home, ".claude", "skills", "dotfiles", "SKILL.md"), nil
}

func claudeDetect() (bool, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return false, fmt.Errorf("resolve home dir: %w", err)
	}
	dir := filepath.Join(home, ".claude", "skills")
	st, err := os.Stat(dir)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return false, nil
		}
		return false, fmt.Errorf("stat %s: %w", dir, err)
	}
	return st.IsDir(), nil
}

var (
	skillInstall bool
	skillAgent   string
	skillForce   bool
)

var skillCmd = &cobra.Command{
	Use:   "skill",
	Short: "Print an agent-installable skill describing this CLI",
	Long: `Print an agent-installable skill describing this CLI.

In the default (stdout) mode, the output is a self-contained markdown
document with YAML frontmatter, covering every available subcommand,
the global flags, the manifest model, and the JSON error envelope.

With --install, the skill is written directly to a known agent's skills
directory. Pass --agent=<name> to target a single agent; supported
values: claude. Existing files are left alone unless --force is set;
byte-identical existing files are reported as "skip" and exit zero.
Use -n/--dry-run to preview install actions without writing.

` + skillJSONShape,
	Example: `  dotfiles skill
  dotfiles skill --json | jq .
  dotfiles skill --install --agent=claude
  dotfiles skill --install --agent=claude --dry-run`,
	RunE: runSkill,
}

func init() {
	skillCmd.Flags().BoolVar(&skillInstall, "install", false, "write the skill to the agent's skills directory instead of stdout")
	skillCmd.Flags().StringVar(&skillAgent, "agent", "", "agent to install the skill into (supported: claude)")
	skillCmd.Flags().BoolVar(&skillForce, "force", false, "overwrite an existing skill file with different content (install mode only)")
	skillCmd.Flags().BoolVarP(&dryRun, "dry-run", "n", false, "preview install actions without writing")
	rootCmd.AddCommand(skillCmd)
}

func runSkill(cmd *cobra.Command, _ []string) error {
	if !skillInstall && (skillAgent != "" || skillForce) {
		return handleErr(cmd, errors.New("--agent and --force require --install"))
	}
	skill := renderSkill(rootCmd)
	if !skillInstall {
		return runSkillStdout(cmd, skill)
	}
	return runSkillInstall(cmd, skill)
}

func runSkillStdout(cmd *cobra.Command, skill Skill) error {
	out := cmd.OutOrStdout()
	if jsonOutput {
		return writeJSON(out, skill)
	}
	_, err := fmt.Fprint(out, skill.Markdown())
	return err
}

func runSkillInstall(cmd *cobra.Command, skill Skill) error {
	targets, err := resolveInstallTargets()
	if err != nil {
		return handleErr(cmd, err)
	}

	actions := make([]installAction, 0, len(targets))
	for _, t := range targets {
		actions = append(actions, planInstall(skill, t, skillForce))
	}
	actions = applyInstall(skill, actions, dryRun)

	out := cmd.OutOrStdout()
	if jsonOutput {
		if err := writeJSON(out, installResponse{Actions: actions}); err != nil {
			return err
		}
	} else {
		for _, a := range actions {
			line := fmt.Sprintf("%s\t%s\t%s", a.Action, a.Agent, a.Path)
			if a.Error != "" {
				line += "\t" + a.Error
			}
			fmt.Fprintln(out, line)
		}
	}

	if installHasFailures(actions) {
		return errSilent
	}
	return nil
}

// planInstall returns the action that running install for target would take,
// without performing any writes. The four action kinds and the OS-error
// fallback are defined on installAction.
func planInstall(skill Skill, target agentTarget, force bool) installAction {
	a := installAction{Agent: target.Name}

	path, err := target.PathFor()
	if err != nil {
		a.Error = err.Error()
		return a
	}
	a.Path = path

	// Pre-check: the agent's containing skills directory must already exist.
	// The command never creates an unfamiliar agent directory.
	skillsDir := filepath.Dir(filepath.Dir(path))
	if st, statErr := os.Stat(skillsDir); statErr != nil || !st.IsDir() {
		a.Error = fmt.Sprintf("agent %q skills directory missing: %s", target.Name, skillsDir)
		return a
	}

	existing, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			a.Action = "create"
			return a
		}
		a.Error = fmt.Sprintf("read %s: %s", path, err)
		return a
	}

	fresh := []byte(skill.Markdown())
	switch {
	case bytes.Equal(existing, fresh):
		a.Action = "skip"
	case force:
		a.Action = "overwrite"
	default:
		a.Action = "conflict"
	}
	return a
}

// applyInstall performs the writes for create/overwrite actions and returns
// the updated action list. Skip/conflict actions and pre-existing per-target
// errors are passed through untouched. In dryRun mode no filesystem writes
// happen and the input is returned unchanged.
func applyInstall(skill Skill, actions []installAction, dryRun bool) []installAction {
	if dryRun {
		return actions
	}
	fresh := []byte(skill.Markdown())
	out := make([]installAction, len(actions))
	copy(out, actions)
	for i := range out {
		a := &out[i]
		if a.Error != "" {
			continue
		}
		if a.Action != "create" && a.Action != "overwrite" {
			continue
		}
		dir := filepath.Dir(a.Path)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			a.Error = fmt.Sprintf("create %s: %s", dir, err)
			continue
		}
		if err := os.WriteFile(a.Path, fresh, 0o644); err != nil {
			a.Error = fmt.Sprintf("write %s: %s", a.Path, err)
			continue
		}
	}
	return out
}

func installHasFailures(actions []installAction) bool {
	for _, a := range actions {
		if a.Error != "" || a.Action == "conflict" {
			return true
		}
	}
	return false
}

// resolveInstallTargets returns the agents to install into.
//
// With --agent=<name>, the result is a single-element slice containing the
// matching registry entry (or an error if no entry matches the name).
//
// Without --agent, every supported agent whose Detect() returns true is
// included; the order follows the registry's declaration order. An empty
// detected list is reported as an actionable error.
func resolveInstallTargets() ([]agentTarget, error) {
	if skillAgent != "" {
		target, ok := findAgent(skillAgent)
		if !ok {
			return nil, fmt.Errorf("unknown agent %q (supported: %s)", skillAgent, supportedAgentNames())
		}
		return []agentTarget{target}, nil
	}

	detected := make([]agentTarget, 0, len(agents))
	for _, a := range agents {
		ok, err := a.Detect()
		if err != nil {
			return nil, fmt.Errorf("detect %s: %w", a.Name, err)
		}
		if ok {
			detected = append(detected, a)
		}
	}
	if len(detected) == 0 {
		return nil, fmt.Errorf("no supported agents detected; pass --agent explicitly (supported: %s)", supportedAgentNames())
	}
	return detected, nil
}

func findAgent(name string) (agentTarget, bool) {
	for _, a := range agents {
		if a.Name == name {
			return a, true
		}
	}
	return agentTarget{}, false
}

func supportedAgentNames() string {
	names := make([]string, 0, len(agents))
	for _, a := range agents {
		names = append(names, a.Name)
	}
	return strings.Join(names, ", ")
}

// renderSkill builds the agent skill from the live Cobra command tree.
// Output is deterministic for a given binary build: commands are sorted by
// name and persistent flags are walked in pflag's lexicographic order.
func renderSkill(root *cobra.Command) Skill {
	var b strings.Builder

	fmt.Fprintf(&b, "# %s\n\n", root.Use)
	fmt.Fprintf(&b, "`%s` manages a checked-in mirror of the user's local config files. A\n", root.Use)
	b.WriteString("JSON manifest (`dotfiles.json`) declares which files to track; the CLI\n")
	b.WriteString("copies (not symlinks) them between the live filesystem and the\n")
	b.WriteString("git-managed mirror.\n\n")

	b.WriteString("## Global flags\n\n")
	b.WriteString("| Flag | Description |\n")
	b.WriteString("|------|-------------|\n")
	root.PersistentFlags().VisitAll(func(f *pflag.Flag) {
		fmt.Fprintf(&b, "| `--%s` | %s |\n", f.Name, f.Usage)
	})
	b.WriteString("\n")

	b.WriteString("## Commands\n\n")
	b.WriteString("Each command's full flag list and JSON output shape is documented in\n")
	fmt.Fprintf(&b, "`%s <command> --help`.\n\n", root.Use)
	b.WriteString("| Command | Purpose |\n")
	b.WriteString("|---------|---------|\n")
	for _, c := range listSkillCommands(root) {
		fmt.Fprintf(&b, "| `%s` | %s |\n", c.Name(), c.Short)
	}
	b.WriteString("\n")

	b.WriteString("## Manifest\n\n")
	b.WriteString("`dotfiles.json` maps a tool name to a list of paths. A trailing `/`\n")
	b.WriteString("marks an entry as a directory whose contents are tracked recursively;\n")
	b.WriteString("without a trailing slash the entry is a single file. The trailing\n")
	b.WriteString("slash is the sole signal for directory-ness and is not inferred from\n")
	b.WriteString("disk: a directory declared without the slash, or a file declared with\n")
	b.WriteString("one, is reported as an error rather than silently guessed. Paths\n")
	b.WriteString("support `~` and absolute paths.\n\n")
	b.WriteString("Example:\n\n")
	b.WriteString("    {\n")
	b.WriteString("      \"git\":   [\"~/.gitconfig\", \"~/.gitignore_global\"],\n")
	b.WriteString("      \"shell\": [\"~/.zshrc\"],\n")
	b.WriteString("      \"nvim\":  [\"~/.config/nvim/\"]\n")
	b.WriteString("    }\n\n")
	b.WriteString("The manifest is the single source of truth: the CLI never touches\n")
	b.WriteString("files outside the resolved live-to-saved mapping.\n\n")

	b.WriteString("## JSON output and errors\n\n")
	b.WriteString("Every command accepts `--json` to emit a single JSON object on stdout.\n")
	b.WriteString("Plain text and JSON are never mixed in the same invocation. The exact\n")
	fmt.Fprintf(&b, "per-command JSON shape is documented in `%s <command> --help`.\n\n", root.Use)
	b.WriteString("On any failure, `--json` mode emits the standard error envelope and\n")
	b.WriteString("the process exits non-zero:\n\n")
	b.WriteString("    { \"error\": { \"message\": \"...\" } }\n")

	return Skill{Name: skillName, Description: skillDescription, Body: b.String()}
}

// listSkillCommands returns the subcommands the skill body enumerates:
// every available command of root except the synthetic `help` and
// `completion` builtins, sorted alphabetically by name for deterministic
// output.
func listSkillCommands(root *cobra.Command) []*cobra.Command {
	out := make([]*cobra.Command, 0, len(root.Commands()))
	for _, c := range root.Commands() {
		if !c.IsAvailableCommand() {
			continue
		}
		switch c.Name() {
		case "help", "completion":
			continue
		}
		out = append(out, c)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name() < out[j].Name() })
	return out
}
