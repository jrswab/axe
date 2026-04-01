package provider

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveCredentials_EnvVars(t *testing.T) {
	t.Setenv("AWS_ACCESS_KEY_ID", "AKID")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "SECRET")
	t.Setenv("AWS_SESSION_TOKEN", "TOKEN")

	creds, err := resolveCredentials("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if creds.AccessKeyID != "AKID" || creds.SecretAccessKey != "SECRET" || creds.SessionToken != "TOKEN" {
		t.Errorf("got %+v", creds)
	}
}

func TestResolveCredentials_EnvVarsNoToken(t *testing.T) {
	t.Setenv("AWS_ACCESS_KEY_ID", "AKID")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "SECRET")

	creds, err := resolveCredentials("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if creds.SessionToken != "" {
		t.Errorf("expected empty session token, got %q", creds.SessionToken)
	}
}

func TestResolveCredentials_File(t *testing.T) {
	dir := t.TempDir()
	credsPath := filepath.Join(dir, "credentials")
	_ = os.WriteFile(credsPath, []byte("[default]\naws_access_key_id = FILEAKID\naws_secret_access_key = FILESECRET\n"), 0600)

	t.Setenv("AWS_ACCESS_KEY_ID", "")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "")
	t.Setenv("AWS_PROFILE", "")
	t.Setenv("AWS_SHARED_CREDENTIALS_FILE", credsPath)

	creds, err := resolveCredentials("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if creds.AccessKeyID != "FILEAKID" || creds.SecretAccessKey != "FILESECRET" {
		t.Errorf("got %+v", creds)
	}
}

func TestResolveCredentials_ProfileSelection(t *testing.T) {
	dir := t.TempDir()
	credsPath := filepath.Join(dir, "credentials")
	_ = os.WriteFile(credsPath, []byte("[default]\naws_access_key_id = DEFAULT\naws_secret_access_key = DEFAULTSECRET\n\n[staging]\naws_access_key_id = STAGING\naws_secret_access_key = STAGINGSECRET\naws_session_token = STAGINGTOKEN\n"), 0600)

	t.Setenv("AWS_ACCESS_KEY_ID", "")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "")
	t.Setenv("AWS_SHARED_CREDENTIALS_FILE", credsPath)

	creds, err := resolveCredentials("staging")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if creds.AccessKeyID != "STAGING" || creds.SecretAccessKey != "STAGINGSECRET" || creds.SessionToken != "STAGINGTOKEN" {
		t.Errorf("got %+v", creds)
	}
}

func TestResolveCredentials_AWSProfileEnv(t *testing.T) {
	dir := t.TempDir()
	credsPath := filepath.Join(dir, "credentials")
	_ = os.WriteFile(credsPath, []byte("[prod]\naws_access_key_id = PROD\naws_secret_access_key = PRODSECRET\n"), 0600)

	t.Setenv("AWS_ACCESS_KEY_ID", "")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "")
	t.Setenv("AWS_SHARED_CREDENTIALS_FILE", credsPath)
	t.Setenv("AWS_PROFILE", "prod")

	creds, err := resolveCredentials("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if creds.AccessKeyID != "PROD" {
		t.Errorf("expected PROD, got %s", creds.AccessKeyID)
	}
}

func TestResolveCredentials_MissingCreds(t *testing.T) {
	t.Setenv("AWS_ACCESS_KEY_ID", "")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "")
	t.Setenv("AWS_SHARED_CREDENTIALS_FILE", "/nonexistent/path")

	_, err := resolveCredentials("")
	if err == nil {
		t.Fatal("expected error for missing credentials")
	}
}

func TestParseCredentialsFile_Comments(t *testing.T) {
	dir := t.TempDir()
	credsPath := filepath.Join(dir, "credentials")
	_ = os.WriteFile(credsPath, []byte("# comment\n; another comment\n[default]\naws_access_key_id = AK\naws_secret_access_key = SK\n"), 0600)

	creds, err := parseCredentialsFile(credsPath, "default")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if creds.AccessKeyID != "AK" {
		t.Errorf("expected AK, got %s", creds.AccessKeyID)
	}
}

func TestParseCredentialsFile_MissingProfile(t *testing.T) {
	dir := t.TempDir()
	credsPath := filepath.Join(dir, "credentials")
	_ = os.WriteFile(credsPath, []byte("[default]\naws_access_key_id = AK\naws_secret_access_key = SK\n"), 0600)

	_, err := parseCredentialsFile(credsPath, "nonexistent")
	if err == nil {
		t.Fatal("expected error for missing profile")
	}
}
