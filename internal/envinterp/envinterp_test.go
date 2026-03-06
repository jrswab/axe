package envinterp

import (
	"reflect"
	"testing"
)

func TestExpandHeaders_StaticValues(t *testing.T) {
	headers := map[string]string{
		"Authorization": "Bearer static-token",
		"X-API-Key":     "abc123",
	}

	got, err := ExpandHeaders(headers)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	want := map[string]string{
		"Authorization": "Bearer static-token",
		"X-API-Key":     "abc123",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestExpandHeaders_SingleVar(t *testing.T) {
	t.Setenv("TOKEN", "secret")
	headers := map[string]string{"Authorization": "Bearer ${TOKEN}"}

	got, err := ExpandHeaders(headers)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if got["Authorization"] != "Bearer secret" {
		t.Fatalf("got %q, want %q", got["Authorization"], "Bearer secret")
	}
}

func TestExpandHeaders_MultipleVarsInOneValue(t *testing.T) {
	t.Setenv("USER", "alice")
	t.Setenv("TOKEN", "abc123")
	headers := map[string]string{"Authorization": "${USER}:${TOKEN}"}

	got, err := ExpandHeaders(headers)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if got["Authorization"] != "alice:abc123" {
		t.Fatalf("got %q, want %q", got["Authorization"], "alice:abc123")
	}
}

func TestExpandHeaders_MultipleHeaders(t *testing.T) {
	t.Setenv("TOKEN", "abc123")
	t.Setenv("ORG", "my-org")
	headers := map[string]string{
		"Authorization": "Bearer ${TOKEN}",
		"X-Org":         "${ORG}",
	}

	got, err := ExpandHeaders(headers)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if got["Authorization"] != "Bearer abc123" {
		t.Fatalf("got %q, want %q", got["Authorization"], "Bearer abc123")
	}
	if got["X-Org"] != "my-org" {
		t.Fatalf("got %q, want %q", got["X-Org"], "my-org")
	}
}

func TestExpandHeaders_MissingVar(t *testing.T) {
	headers := map[string]string{"Authorization": "Bearer ${MISSING_TOKEN}"}

	_, err := ExpandHeaders(headers)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if got, want := err.Error(), `header "Authorization": environment variable "MISSING_TOKEN" is not set or empty`; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestExpandHeaders_EmptyVar(t *testing.T) {
	t.Setenv("TOKEN", "")
	headers := map[string]string{"Authorization": "Bearer ${TOKEN}"}

	_, err := ExpandHeaders(headers)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if got, want := err.Error(), `header "Authorization": environment variable "TOKEN" is not set or empty`; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestExpandHeaders_NilMap(t *testing.T) {
	got, err := ExpandHeaders(nil)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if got != nil {
		t.Fatalf("got %v, want nil", got)
	}
}

func TestExpandHeaders_EmptyMap(t *testing.T) {
	headers := map[string]string{}
	got, err := ExpandHeaders(headers)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if got == nil {
		t.Fatal("got nil map, want empty non-nil map")
	}
	if len(got) != 0 {
		t.Fatalf("got len %d, want 0", len(got))
	}
}

func TestExpandHeaders_NoPattern(t *testing.T) {
	headers := map[string]string{"X-Plain": "abc ${ not a token"}

	got, err := ExpandHeaders(headers)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if got["X-Plain"] != "abc ${ not a token" {
		t.Fatalf("got %q, want %q", got["X-Plain"], "abc ${ not a token")
	}
}

func TestExpandHeaders_DollarWithoutBrace(t *testing.T) {
	t.Setenv("TOKEN", "secret")
	headers := map[string]string{"Authorization": "Bearer $TOKEN"}

	got, err := ExpandHeaders(headers)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if got["Authorization"] != "Bearer $TOKEN" {
		t.Fatalf("got %q, want %q", got["Authorization"], "Bearer $TOKEN")
	}
}

func TestExpandHeaders_InputNotModified(t *testing.T) {
	t.Setenv("TOKEN", "secret")
	headers := map[string]string{"Authorization": "Bearer ${TOKEN}"}

	_, err := ExpandHeaders(headers)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if headers["Authorization"] != "Bearer ${TOKEN}" {
		t.Fatalf("input map modified: got %q", headers["Authorization"])
	}
}
