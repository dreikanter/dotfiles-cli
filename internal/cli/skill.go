package cli

import (
	"fmt"
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

var skillCmd = &cobra.Command{
	Use:   "skill",
	Short: "Print an agent-installable skill describing this CLI",
	Long: `Print an agent-installable skill describing this CLI.

The output is a self-contained markdown document with YAML frontmatter
(name, description) followed by a body covering every available
subcommand, the global flags, the manifest model, and the JSON error
envelope. Pipe it into your AI agent's skill location.

` + skillJSONShape,
	Example: `  dotfiles skill
  dotfiles skill --json | jq .`,
	RunE: func(cmd *cobra.Command, _ []string) error {
		skill := renderSkill(rootCmd)
		out := cmd.OutOrStdout()
		if jsonOutput {
			return writeJSON(out, skill)
		}
		_, err := fmt.Fprint(out, skill.Markdown())
		return err
	},
}

func init() {
	rootCmd.AddCommand(skillCmd)
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
	fmt.Fprintf(&b, "Each command's full flag list and JSON output shape is documented in\n")
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
	b.WriteString("without a trailing slash the entry is a single file. Paths support `~`\n")
	b.WriteString("and absolute paths.\n\n")
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
