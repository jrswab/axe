package provider

import (
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestSignRequest_AuthorizationHeader(t *testing.T) {
	creds := &awsCredentials{AccessKeyID: "AKIDEXAMPLE", SecretAccessKey: "wJalrXUtnFEMI/K7MDENG+bPxRfiCYEXAMPLEKEY"}
	body := []byte(`{"messages":[]}`)
	req, _ := http.NewRequest("POST", "https://bedrock-runtime.us-east-1.amazonaws.com/model/test/converse", nil)
	req.Header.Set("Content-Type", "application/json")

	now := time.Date(2024, 1, 15, 12, 0, 0, 0, time.UTC)
	signRequest(req, creds, "us-east-1", "bedrock", body, now)

	auth := req.Header.Get("Authorization")
	if !strings.HasPrefix(auth, "AWS4-HMAC-SHA256 Credential=AKIDEXAMPLE/20240115/us-east-1/bedrock/aws4_request") {
		t.Errorf("unexpected Authorization header: %s", auth)
	}
	if !strings.Contains(auth, "SignedHeaders=") {
		t.Error("missing SignedHeaders in Authorization")
	}
	if !strings.Contains(auth, "Signature=") {
		t.Error("missing Signature in Authorization")
	}
}

func TestSignRequest_WithSessionToken(t *testing.T) {
	creds := &awsCredentials{AccessKeyID: "AKID", SecretAccessKey: "SECRET", SessionToken: "TOKEN"}
	body := []byte(`{}`)
	req, _ := http.NewRequest("POST", "https://bedrock-runtime.us-west-2.amazonaws.com/model/test/converse", nil)

	now := time.Date(2024, 6, 1, 0, 0, 0, 0, time.UTC)
	signRequest(req, creds, "us-west-2", "bedrock", body, now)

	if req.Header.Get("x-amz-security-token") != "TOKEN" {
		t.Error("expected x-amz-security-token header")
	}
	if req.Header.Get("x-amz-date") != "20240601T000000Z" {
		t.Errorf("unexpected x-amz-date: %s", req.Header.Get("x-amz-date"))
	}
}

func TestSignRequest_NoSessionToken(t *testing.T) {
	creds := &awsCredentials{AccessKeyID: "AKID", SecretAccessKey: "SECRET"}
	body := []byte(`{}`)
	req, _ := http.NewRequest("POST", "https://bedrock-runtime.us-east-1.amazonaws.com/model/test/converse", nil)

	signRequest(req, creds, "us-east-1", "bedrock", body, time.Now())

	if req.Header.Get("x-amz-security-token") != "" {
		t.Error("expected no x-amz-security-token header")
	}
}

func TestSignRequest_Deterministic(t *testing.T) {
	creds := &awsCredentials{AccessKeyID: "AKID", SecretAccessKey: "SECRET"}
	body := []byte(`{"test":true}`)
	now := time.Date(2024, 3, 15, 10, 30, 0, 0, time.UTC)

	req1, _ := http.NewRequest("POST", "https://bedrock-runtime.us-east-1.amazonaws.com/model/m/converse", nil)
	req1.Header.Set("Content-Type", "application/json")
	signRequest(req1, creds, "us-east-1", "bedrock", body, now)

	req2, _ := http.NewRequest("POST", "https://bedrock-runtime.us-east-1.amazonaws.com/model/m/converse", nil)
	req2.Header.Set("Content-Type", "application/json")
	signRequest(req2, creds, "us-east-1", "bedrock", body, now)

	if req1.Header.Get("Authorization") != req2.Header.Get("Authorization") {
		t.Error("signing is not deterministic")
	}
}

func TestDeriveSigningKey(t *testing.T) {
	key := deriveSigningKey("wJalrXUtnFEMI/K7MDENG+bPxRfiCYEXAMPLEKEY", "20120215", "us-east-1", "iam")
	if len(key) != 32 {
		t.Errorf("expected 32-byte key, got %d", len(key))
	}
}
