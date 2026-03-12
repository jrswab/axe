package provider

import (
	"testing"
)

func TestNewBedrock_EmptyRegion(t *testing.T) {
	_, err := NewBedrock("")
	if err == nil {
		t.Fatal("expected error for empty region")
	}
}

func TestNewBedrock_ValidRegion(t *testing.T) {
	b, err := NewBedrock("us-east-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if b.region != "us-east-1" {
		t.Errorf("expected region us-east-1, got %s", b.region)
	}
	if b.client == nil {
		t.Error("expected client to be initialized")
	}
}

func TestNewBedrock_WithBedrockRegion(t *testing.T) {
	b, err := NewBedrock("us-west-2", WithBedrockRegion("eu-west-1"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if b.region != "eu-west-1" {
		t.Errorf("expected region eu-west-1, got %s", b.region)
	}
}
