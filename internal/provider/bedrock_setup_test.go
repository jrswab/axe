package provider

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
)

func TestNewBedrock_RegionValidation(t *testing.T) {
	tests := []struct {
		name   string
		region string
		option string
		valid  bool
	}{
		{name: "standard region", region: "us-east-1", valid: true},
		{name: "63-character label", region: strings.Repeat("a", 63), valid: true},
		{name: "64-character label", region: strings.Repeat("a", 64)},
		{name: "leading hyphen", region: "-us-east-1"},
		{name: "trailing hyphen", region: "us-east-1-"},
		{name: "uppercase", region: "US-EAST-1"},
		{name: "underscore", region: "us_east_1"},
		{name: "dot redirects host", region: "x.attacker.example"},
		{name: "slash redirects path", region: "x/attacker"},
		{name: "at sign redirects userinfo", region: "x@attacker"},
		{name: "colon adds port", region: "x:443"},
		{name: "hash adds fragment", region: "x#"},
		{name: "percent escape", region: "x%2e"},
		{name: "whitespace", region: "us east 1"},
		{name: "invalid region from option", region: "us-east-1", option: "x.attacker.example#"},
		{name: "valid region from option", region: "us_east_1", option: "eu-west-1", valid: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := []BedrockOption{withBedrockCreds(testCreds())}
			if tt.option != "" {
				opts = append(opts, WithBedrockRegion(tt.option))
			}
			b, err := NewBedrock(tt.region, opts...)
			if tt.valid {
				if err != nil {
					t.Fatalf("NewBedrock(%q) error = %v, want nil", tt.region, err)
				}
				return
			}
			if !errors.Is(err, errInvalidBedrockRegion) || b != nil {
				t.Fatalf("NewBedrock(%q, option %q) = %v, %v; want errInvalidBedrockRegion", tt.region, tt.option, b, err)
			}
		})
	}
}

func TestNewBedrock_InvalidRegionSkipsCredentialLoading(t *testing.T) {
	t.Setenv("AWS_ACCESS_KEY_ID", "")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "")
	t.Setenv("AWS_SHARED_CREDENTIALS_FILE", t.TempDir()+"/missing-credentials")
	if _, err := NewBedrock("x.attacker.example"); !errors.Is(err, errInvalidBedrockRegion) {
		t.Fatalf("NewBedrock error = %v, want errInvalidBedrockRegion before credential loading", err)
	}
}

func TestBedrockSetupDiagnostic(t *testing.T) {
	const guidance = "AWS credentials not found: set AWS_ACCESS_KEY_ID and AWS_SECRET_ACCESS_KEY, or configure a credentials file and AWS_PROFILE; if AWS_SHARED_CREDENTIALS_FILE is set, correct or unset it"
	const path = "/home/user/.aws/team-credentials"
	tests := []struct {
		name string
		err  error
		want string
	}{
		{name: "invalid region", err: errInvalidBedrockRegion, want: errInvalidBedrockRegion.Error()},
		{name: "safe profile", err: &awsProfileError{profile: "dev_team-1", path: path}, want: guidance + `: profile "dev_team-1" not found or missing credentials`},
		{name: "wrapped safe profile", err: fmt.Errorf("outer: %w", &awsProfileError{profile: "default", path: path}), want: guidance + `: profile "default" not found or missing credentials`},
		{name: "profile contains file basename", err: &awsProfileError{profile: "x-Team-Credentials", path: path}, want: guidance},
		{name: "profile with unsafe characters", err: &awsProfileError{profile: "../etc/passwd", path: path}, want: guidance},
		{name: "non-ASCII profile", err: &awsProfileError{profile: "prodé", path: path}, want: guidance},
		{name: "over-long profile", err: &awsProfileError{profile: strings.Repeat("p", 65), path: path}, want: guidance},
		{name: "empty profile", err: &awsProfileError{profile: "", path: path}, want: guidance},
		{name: "missing file", err: &os.PathError{Op: "open", Path: path, Err: os.ErrNotExist}, want: guidance},
		{name: "permission denied", err: &os.PathError{Op: "open", Path: path, Err: os.ErrPermission}, want: guidance},
		{name: "scanner error", err: errors.New("bufio.Scanner: token too long"), want: guidance},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := BedrockSetupDiagnostic(tt.err)
			if got != tt.want {
				t.Fatalf("BedrockSetupDiagnostic() = %q, want %q", got, tt.want)
			}
			if strings.Contains(got, path) || strings.Contains(strings.ToLower(got), "team-credentials") {
				t.Fatalf("BedrockSetupDiagnostic() = %q leaks credential-file details", got)
			}
		})
	}
}

func TestConvertMessages_BedrockRoles(t *testing.T) {
	tests := []struct {
		role string
		want string
	}{
		{role: "user", want: "user"},
		{role: "assistant", want: "assistant"},
		{role: "tool", want: "user"},
		{role: "system", want: "assistant"},
		{role: "", want: "assistant"},
	}
	for _, tt := range tests {
		t.Run(fmt.Sprintf("role %q", tt.role), func(t *testing.T) {
			got := convertMessages([]Message{{Role: tt.role, Content: "x"}})
			if len(got) != 1 || got[0].Role != tt.want {
				t.Fatalf("convertMessages(role %q) = %+v, want role %q", tt.role, got, tt.want)
			}
		})
	}
}
