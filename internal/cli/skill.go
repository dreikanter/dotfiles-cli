package cli

import (
	"bytes"
	_ "embed"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

//go:embed skill.md
var skillContent string

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
	skill := loadSkill()
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

// loadSkill parses the embedded skill.md into a Skill. The file begins with a
// YAML frontmatter block containing name and description, followed by the
// markdown body.
func loadSkill() Skill {
	const openDelim = "---\n"
	const closeDelim = "\n---\n"
	if !strings.HasPrefix(skillContent, openDelim) {
		panic("skill.md: missing opening frontmatter delimiter")
	}
	rest := skillContent[len(openDelim):]
	idx := strings.Index(rest, closeDelim)
	if idx < 0 {
		panic("skill.md: missing closing frontmatter delimiter")
	}
	fm := rest[:idx]
	body := strings.TrimPrefix(rest[idx+len(closeDelim):], "\n")

	var name, description string
	for _, line := range strings.Split(fm, "\n") {
		if v, ok := strings.CutPrefix(line, "name: "); ok {
			name = v
		} else if v, ok := strings.CutPrefix(line, "description: "); ok {
			description = v
		}
	}
	return Skill{Name: name, Description: description, Body: body}
}
