package cmd

import (
	"bytes"
	"testing"
)

func TestVersionCommand_Output(t *testing.T) {
	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetArgs([]string{"version"})

	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got := buf.String()
	want := "axe version 0.1.0\n"
	if got != want {
		t.Errorf("version output = %q, want %q", got, want)
	}
}

func TestVersionConstant(t *testing.T) {
	if Version != "0.1.0" {
		t.Errorf("Version = %q, want %q", Version, "0.1.0")
	}
}
