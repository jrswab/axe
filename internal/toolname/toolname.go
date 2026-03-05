package toolname

// Tool name constants for agent configuration.
// These are the canonical string values used in TOML config files
// and tool dispatch logic.
const (
	CallAgent     = "call_agent"
	ListDirectory = "list_directory"
	ReadFile      = "read_file"
	WriteFile     = "write_file"
	EditFile      = "edit_file"
	RunCommand    = "run_command"
	URLFetch      = "url_fetch"
)

// ValidNames returns the set of tool names that can appear in an agent's
// tools configuration field. Each call returns a new map instance.
// CallAgent is excluded because it is controlled by the sub_agents field.
func ValidNames() map[string]bool {
	return map[string]bool{
		ListDirectory: true,
		ReadFile:      true,
		WriteFile:     true,
		EditFile:      true,
		RunCommand:    true,
		URLFetch:      true,
	}
}
