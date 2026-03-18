package provider

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"
)

// signRequest signs an HTTP request using AWS Signature Version 4.
func signRequest(req *http.Request, creds *awsCredentials, region, service string, body []byte, now time.Time) {
	// Timestamps
	amzDate := now.UTC().Format("20060102T150405Z")
	dateStamp := now.UTC().Format("20060102")

	// Payload hash
	payloadHash := sha256Hex(body)

	// Set required headers before building canonical request
	req.Header.Set("x-amz-date", amzDate)
	req.Header.Set("x-amz-content-sha256", payloadHash)
	if creds.SessionToken != "" {
		req.Header.Set("x-amz-security-token", creds.SessionToken)
	}
	if req.Header.Get("Host") == "" {
		req.Header.Set("Host", req.URL.Host)
	}

	// Signed headers (sorted lowercase)
	signedHeaders := sortedHeaderNames(req.Header)
	signedHeadersStr := strings.Join(signedHeaders, ";")

	// Canonical request
	canonicalReq := strings.Join([]string{
		req.Method,
		req.URL.Path,
		req.URL.RawQuery,
		canonicalHeaders(req.Header, signedHeaders),
		signedHeadersStr,
		payloadHash,
	}, "\n")

	// Credential scope
	scope := dateStamp + "/" + region + "/" + service + "/aws4_request"

	// String to sign
	stringToSign := "AWS4-HMAC-SHA256\n" + amzDate + "\n" + scope + "\n" + sha256Hex([]byte(canonicalReq))

	// Signing key
	signingKey := deriveSigningKey(creds.SecretAccessKey, dateStamp, region, service)

	// Signature
	signature := hex.EncodeToString(hmacSHA256(signingKey, []byte(stringToSign)))

	// Authorization header
	req.Header.Set("Authorization", fmt.Sprintf(
		"AWS4-HMAC-SHA256 Credential=%s/%s, SignedHeaders=%s, Signature=%s",
		creds.AccessKeyID, scope, signedHeadersStr, signature,
	))
}

// deriveSigningKey computes the SigV4 signing key from the secret and scope components.
func deriveSigningKey(secret, dateStamp, region, service string) []byte {
	kDate := hmacSHA256([]byte("AWS4"+secret), []byte(dateStamp))
	kRegion := hmacSHA256(kDate, []byte(region))
	kService := hmacSHA256(kRegion, []byte(service))
	return hmacSHA256(kService, []byte("aws4_request"))
}

// hmacSHA256 computes an HMAC-SHA256 digest.
func hmacSHA256(key, data []byte) []byte {
	h := hmac.New(sha256.New, key)
	h.Write(data)
	return h.Sum(nil)
}

// sha256Hex returns the hex-encoded SHA-256 hash of data.
func sha256Hex(data []byte) string {
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}

// sortedHeaderNames returns lowercase header names sorted alphabetically.
func sortedHeaderNames(h http.Header) []string {
	var names []string
	for k := range h {
		names = append(names, strings.ToLower(k))
	}
	sort.Strings(names)
	return names
}

// canonicalHeaders builds the canonical headers string for SigV4 signing.
func canonicalHeaders(h http.Header, sorted []string) string {
	var b strings.Builder
	for _, name := range sorted {
		b.WriteString(name)
		b.WriteByte(':')
		b.WriteString(strings.TrimSpace(h.Get(name)))
		b.WriteByte('\n')
	}
	return b.String()
}
