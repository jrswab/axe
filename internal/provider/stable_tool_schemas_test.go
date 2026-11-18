package provider

// stableToolSchemaFixture constructs equivalent definitions in opposite parameter
// insertion orders. Wire expectations belong to each adapter's test, not here.
func stableToolSchemaFixture(reverse bool) []Tool {
	parameters := [][]struct {
		name       string
		definition ToolParameter
	}{
		{
			{"account", ToolParameter{Type: "string", Description: "Account identifier", Required: true}},
			{"key", ToolParameter{Type: "string", Description: "Record key", Required: true}},
			{"limit", ToolParameter{Type: "integer", Description: "Maximum records"}},
			{"region", ToolParameter{Type: "string", Description: "Lookup region", Required: true}},
			{"verbose", ToolParameter{Type: "boolean", Description: "Include details"}},
		},
		{
			{"format", ToolParameter{Type: "string", Description: "Report format", Required: true}},
			{"ratio", ToolParameter{Type: "number", Description: "Sampling ratio"}},
			{"record", ToolParameter{Type: "string", Description: "Record to report", Required: true}},
			{"target", ToolParameter{Type: "string", Description: "Report destination", Required: true}},
			{"title", ToolParameter{Type: "string", Description: "Optional title"}},
		},
	}
	tools := []Tool{
		{Name: "z_lookup", Description: "Look up a record"},
		{Name: "a_report", Description: "Report a record"},
	}
	for i, entries := range parameters {
		tools[i].Parameters = make(map[string]ToolParameter, len(entries))
		for j := range entries {
			if reverse {
				j = len(entries) - 1 - j
			}
			tools[i].Parameters[entries[j].name] = entries[j].definition
		}
	}
	return tools
}
