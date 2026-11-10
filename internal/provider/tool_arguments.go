package provider

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
)

const (
	minIntegerDigits      = 7
	maxPlainIntegerDigits = 309
)

// DecodeJSON preserves json.Unmarshal validation for the exact destination type,
// including overwritten values, while retaining interface numbers as json.Number.
// T must not contain json.Unmarshaler types: they would run twice, and UseNumber
// would not reach them.
func DecodeJSON[T any](data []byte, dst *T) error {
	var legacy T
	if err := json.Unmarshal(data, &legacy); err != nil {
		return err
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	if err := decoder.Decode(dst); err != nil {
		return err
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		if err != nil {
			return err
		}
		return errors.New("unexpected JSON value after first value")
	}
	return nil
}

// FormatToolArgument keeps eligible exact top-level integers in plain digits.
// All other values retain the legacy float64-derived formatting.
// Input must have been validated by DecodeJSON before formatting.
func FormatToolArgument(value any) string {
	if number, ok := value.(json.Number); ok {
		legacy, _ := number.Float64() // DecodeJSON already preserves range errors.
		if digits, ok := toolIntegerDigits(string(number)); ok {
			if normalized, err := strconv.ParseFloat(digits, 64); err == nil && normalized == legacy {
				return digits
			}
		}
		return fmt.Sprintf("%v", legacy)
	}
	return fmt.Sprintf("%v", legacyToolValue(value))
}

// toolIntegerDigits uses coefficient lengths, not float64 rounding, to decide
// integrality and the threshold. It never expands more than 309 result digits.
func toolIntegerDigits(token string) (string, bool) {
	sign := ""
	if strings.HasPrefix(token, "-") {
		sign, token = "-", token[1:]
	}
	mantissa, exponent, hasExponent := strings.Cut(strings.ToLower(token), "e")
	exp := 0
	if hasExponent {
		var err error
		exp, err = strconv.Atoi(exponent)
		if err != nil {
			// An exponent outside int cannot cancel an input-sized coefficient.
			return "", false
		}
	}
	fractionDigits := 0
	if dot := strings.IndexByte(mantissa, '.'); dot >= 0 {
		fractionDigits = len(mantissa) - dot - 1
		mantissa = mantissa[:dot] + mantissa[dot+1:]
	}
	coefficient := strings.TrimLeft(mantissa, "0")
	if coefficient == "" {
		return "", false
	}
	// Check bounds before subtraction so even extreme int exponents are safe.
	if exp < fractionDigits-len(coefficient) || exp > fractionDigits+maxPlainIntegerDigits {
		return "", false
	}
	shift := exp - fractionDigits
	digits := len(coefficient) + shift
	// deliberate: 309 result digits; larger results retain legacy formatting.
	if digits < minIntegerDigits || digits > maxPlainIntegerDigits {
		return "", false
	}
	if shift < 0 {
		if strings.Trim(coefficient[digits:], "0") != "" {
			return "", false
		}
		return sign + coefficient[:digits], true
	}
	return sign + coefficient + strings.Repeat("0", shift), true
}

func legacyToolValue(value any) any {
	switch v := value.(type) {
	case json.Number:
		number, _ := v.Float64()
		return number
	case map[string]interface{}:
		result := make(map[string]interface{}, len(v))
		for key, item := range v {
			result[key] = legacyToolValue(item)
		}
		return result
	case []interface{}:
		result := make([]interface{}, len(v))
		for i, item := range v {
			result[i] = legacyToolValue(item)
		}
		return result
	default:
		return value
	}
}
