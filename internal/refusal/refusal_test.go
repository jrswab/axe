package refusal

import (
	"strings"
	"testing"
)

func TestDetect_EmptyString(t *testing.T) {
	if Detect("") {
		t.Fatal("Detect(\"\") = true, want false")
	}
}

func TestDetect_NormalContent(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		content string
	}{
		{name: "helpful greeting", content: "Hello, how can I help you today?"},
		{name: "solution statement", content: "Here is the solution to your problem."},
		{name: "function description", content: "The function returns true when the input is valid."},
		{name: "unable without first person", content: "Unable to connect to the database."},
		{name: "third person cannot", content: "The AI cannot process this."},
		{name: "can but not cannot", content: "I can notify you when it's ready."},
		{name: "as an ai non refusal", content: "As an AI, I'm happy to help with that!"},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if Detect(tt.content) {
				t.Fatalf("Detect(%q) = true, want false", tt.content)
			}
		})
	}
}

func TestDetect_SimpleRefusals(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		content string
	}{
		{name: "i cannot", content: "I cannot assist with that request."},
		{name: "i cant", content: "I can't help with that."},
		{name: "im unable to", content: "I'm unable to process this request."},
		{name: "i am unable to", content: "I am unable to fulfill this."},
		{name: "im not able to", content: "I'm not able to do that."},
		{name: "i am not able to", content: "I am not able to do that."},
		{name: "i must decline", content: "I must decline this request."},
		{name: "i dont have the ability", content: "I don't have the ability to do that."},
		{name: "i do not have the ability", content: "I do not have the ability to help."},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if !Detect(tt.content) {
				t.Fatalf("Detect(%q) = false, want true", tt.content)
			}
		})
	}
}

func TestDetect_CaseInsensitive(t *testing.T) {
	t.Parallel()

	tests := []string{
		"I CANNOT do that.",
		"i cannot do that.",
		"I Can't Help With That.",
		"I'M UNABLE TO ASSIST.",
	}

	for _, content := range tests {
		content := content
		t.Run(content, func(t *testing.T) {
			t.Parallel()

			if !Detect(content) {
				t.Fatalf("Detect(%q) = false, want true", content)
			}
		})
	}
}

func TestDetect_RefusalAfterPreamble(t *testing.T) {
	t.Parallel()

	tests := []string{
		"I appreciate your question. However, I cannot help with that.",
		"Thank you for asking. Unfortunately, I'm unable to process this request.",
		"I understand your concern, but I must decline.",
		"I'm sorry, but I can't assist with that request.",
	}

	for _, content := range tests {
		content := content
		t.Run(content, func(t *testing.T) {
			t.Parallel()

			if !Detect(content) {
				t.Fatalf("Detect(%q) = false, want true", content)
			}
		})
	}
}

func TestDetect_AsAnAI_Compound(t *testing.T) {
	t.Parallel()

	tests := []string{
		"As an AI, I cannot provide medical advice.",
		"As an AI language model, I'm unable to help with that.",
		"As an AI assistant, I must decline this request.",
		"As an AI, I am not able to do this.",
	}

	for _, content := range tests {
		content := content
		t.Run(content, func(t *testing.T) {
			t.Parallel()

			if !Detect(content) {
				t.Fatalf("Detect(%q) = false, want true", content)
			}
		})
	}
}

func TestDetect_AsAnAI_NoRefusalIndicator(t *testing.T) {
	t.Parallel()

	tests := []string{
		"As an AI, I'm happy to help you with this task.",
		"As an AI language model, I have access to a wide range of information.",
	}

	for _, content := range tests {
		content := content
		t.Run(content, func(t *testing.T) {
			t.Parallel()

			if Detect(content) {
				t.Fatalf("Detect(%q) = true, want false", content)
			}
		})
	}
}

func TestDetect_WhitespaceOnly(t *testing.T) {
	if Detect("   \n\t  ") {
		t.Fatal("Detect(whitespace) = true, want false")
	}
}

func TestDetect_LongContent_RefusalAtEnd(t *testing.T) {
	prefix := strings.Repeat("a", 10000)
	content := prefix + " I cannot help with that."

	if !Detect(content) {
		t.Fatal("Detect(long content with refusal at end) = false, want true")
	}
}

func TestDetect_LongContent_NoRefusal(t *testing.T) {
	content := strings.Repeat("a", 10000)

	if Detect(content) {
		t.Fatal("Detect(long content with no refusal) = true, want false")
	}
}
