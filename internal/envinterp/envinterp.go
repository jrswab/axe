package envinterp

import (
	"fmt"
	"os"
	"regexp"
)

var envPattern = regexp.MustCompile(`\$\{([A-Za-z_][A-Za-z0-9_]*)\}`)

// ExpandHeaders expands ${VAR} references in header values.
func ExpandHeaders(headers map[string]string) (map[string]string, error) {
	if headers == nil {
		return nil, nil
	}

	expanded := make(map[string]string, len(headers))
	for key, value := range headers {
		replaced, err := expandValue(value)
		if err != nil {
			return nil, fmt.Errorf("header %q: %w", key, err)
		}
		expanded[key] = replaced
	}

	return expanded, nil
}

func expandValue(value string) (string, error) {
	matches := envPattern.FindAllStringSubmatch(value, -1)
	if len(matches) == 0 {
		return value, nil
	}

	var expandErr error
	replaced := envPattern.ReplaceAllStringFunc(value, func(token string) string {
		if expandErr != nil {
			return token
		}
		parts := envPattern.FindStringSubmatch(token)
		if len(parts) != 2 {
			return token
		}
		name := parts[1]
		v := os.Getenv(name)
		if v == "" {
			expandErr = fmt.Errorf("environment variable %q is not set or empty", name)
			return token
		}
		return v
	})

	if expandErr != nil {
		return "", expandErr
	}

	return replaced, nil
}
