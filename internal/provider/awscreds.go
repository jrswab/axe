package provider

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// awsCredentials holds AWS access credentials for request signing.
type awsCredentials struct {
	AccessKeyID     string
	SecretAccessKey string
	SessionToken    string
}

type awsProfileError struct {
	profile string
	path    string
}

func (e *awsProfileError) Error() string {
	return fmt.Sprintf("profile %q not found or missing credentials", e.profile)
}

// BedrockSetupDiagnostic projects a Bedrock construction error without exposing local credential-file details.
// @anchor:provider:policy Keep Nested Bedrock setup diagnostics free of local credential-file details.
func BedrockSetupDiagnostic(err error) string {
	if errors.Is(err, errInvalidBedrockRegion) {
		return errInvalidBedrockRegion.Error()
	}
	const guidance = "AWS credentials not found: set AWS_ACCESS_KEY_ID and AWS_SECRET_ACCESS_KEY, or configure a credentials file and AWS_PROFILE"
	var profileErr *awsProfileError
	if errors.As(err, &profileErr) && safeAWSProfileName(profileErr.profile, profileErr.path) {
		return guidance + ": " + profileErr.Error()
	}
	return guidance
}

func safeAWSProfileName(profile, path string) bool {
	if len(profile) == 0 || len(profile) > 64 || strings.Contains(strings.ToLower(profile), strings.ToLower(filepath.Base(path))) {
		return false
	}
	for _, c := range profile {
		if (c < 'a' || c > 'z') && (c < 'A' || c > 'Z') && (c < '0' || c > '9') && c != '-' && c != '_' {
			return false
		}
	}
	return true
}

// resolveCredentials loads AWS credentials from env vars first, then ~/.aws/credentials.
func resolveCredentials(profile string) (*awsCredentials, error) {
	if ak, sk := os.Getenv("AWS_ACCESS_KEY_ID"), os.Getenv("AWS_SECRET_ACCESS_KEY"); ak != "" && sk != "" {
		return &awsCredentials{AccessKeyID: ak, SecretAccessKey: sk, SessionToken: os.Getenv("AWS_SESSION_TOKEN")}, nil
	}

	if profile == "" {
		profile = os.Getenv("AWS_PROFILE")
	}
	if profile == "" {
		profile = "default"
	}

	credsFile := os.Getenv("AWS_SHARED_CREDENTIALS_FILE")
	if credsFile == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("AWS credentials not found: set AWS_ACCESS_KEY_ID and AWS_SECRET_ACCESS_KEY")
		}
		credsFile = filepath.Join(home, ".aws", "credentials")
	}

	creds, err := parseCredentialsFile(credsFile, profile)
	if err != nil {
		return nil, fmt.Errorf("AWS credentials not found: set AWS_ACCESS_KEY_ID and AWS_SECRET_ACCESS_KEY, or configure ~/.aws/credentials: %w", err)
	}
	return creds, nil
}

// parseCredentialsFile reads AWS credentials for the given profile from an INI-format file.
func parseCredentialsFile(path, profile string) (*awsCredentials, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()

	var creds awsCredentials
	inProfile := false
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || line[0] == '#' || line[0] == ';' {
			continue
		}
		if line[0] == '[' && line[len(line)-1] == ']' {
			inProfile = strings.TrimSpace(line[1:len(line)-1]) == profile
			continue
		}
		if !inProfile {
			continue
		}
		if k, v, ok := strings.Cut(line, "="); ok {
			switch strings.TrimSpace(k) {
			case "aws_access_key_id":
				creds.AccessKeyID = strings.TrimSpace(v)
			case "aws_secret_access_key":
				creds.SecretAccessKey = strings.TrimSpace(v)
			case "aws_session_token":
				creds.SessionToken = strings.TrimSpace(v)
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	if creds.AccessKeyID == "" || creds.SecretAccessKey == "" {
		return nil, &awsProfileError{profile: profile, path: path}
	}
	return &creds, nil
}
