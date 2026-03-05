package toolname

import "testing"

func TestValidNames_ReturnsExpectedCount(t *testing.T) {
	names := ValidNames()
	if len(names) != 6 {
		t.Errorf("ValidNames() returned %d entries, want 6", len(names))
	}
}

func TestValidNames_ContainsAllExpectedNames(t *testing.T) {
	names := ValidNames()
	expected := []string{ListDirectory, ReadFile, WriteFile, EditFile, RunCommand, URLFetch}
	for _, name := range expected {
		if !names[name] {
			t.Errorf("ValidNames() missing %q", name)
		}
	}
}

func TestValidNames_ExcludesCallAgent(t *testing.T) {
	names := ValidNames()
	if names[CallAgent] {
		t.Error("ValidNames() should not contain call_agent")
	}
}

func TestValidNames_ReturnsNewMapEachCall(t *testing.T) {
	first := ValidNames()
	first["injected"] = true

	second := ValidNames()
	if second["injected"] {
		t.Error("ValidNames() returned a shared map; modifying one affected the other")
	}
}

func TestConstants_Values(t *testing.T) {
	tests := []struct {
		name     string
		got      string
		expected string
	}{
		{"CallAgent", CallAgent, "call_agent"},
		{"ListDirectory", ListDirectory, "list_directory"},
		{"ReadFile", ReadFile, "read_file"},
		{"WriteFile", WriteFile, "write_file"},
		{"EditFile", EditFile, "edit_file"},
		{"RunCommand", RunCommand, "run_command"},
		{"URLFetch", URLFetch, "url_fetch"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.got != tt.expected {
				t.Errorf("%s = %q, want %q", tt.name, tt.got, tt.expected)
			}
		})
	}
}
