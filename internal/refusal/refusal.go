package refusal

import "strings"

var simplePatterns = []string{
	"i cannot",
	"i can't",
	"i'm unable to",
	"i am unable to",
	"i'm not able to",
	"i am not able to",
	"i must decline",
	"i don't have the ability",
	"i do not have the ability",
}

var compoundIndicators = []string{
	"i cannot",
	"i can't",
	"i'm unable",
	"i am unable",
	"i'm not able",
	"i am not able",
	"i must decline",
}

func Detect(content string) bool {
	if content == "" {
		return false
	}

	lowerContent := strings.ToLower(content)

	for _, pattern := range simplePatterns {
		if containsIndicator(lowerContent, pattern) {
			return true
		}
	}

	if strings.Contains(lowerContent, "as an ai") {
		for _, indicator := range compoundIndicators {
			if containsIndicator(lowerContent, indicator) {
				return true
			}
		}
	}

	return false
}

func containsIndicator(content, indicator string) bool {
	start := 0
	for {
		idx := strings.Index(content[start:], indicator)
		if idx == -1 {
			return false
		}

		idx += start
		if idx == 0 || !isASCIILetter(content[idx-1]) {
			return true
		}

		start = idx + 1
	}
}

func isASCIILetter(b byte) bool {
	return (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z')
}
