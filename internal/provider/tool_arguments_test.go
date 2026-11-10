package provider

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
)

func TestLargeIntegerArgs_Formatter(t *testing.T) {
	// Literal expectations are independent of the formatter and float64 rounding.
	const power308 = "100000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000"
	const maxFiniteInteger = "179769313486231570000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000"
	tests := []struct {
		name    string
		raw     string
		want    string
		wantErr bool
	}{
		{name: "zero", raw: `0`, want: "0"},
		{name: "negative zero", raw: `-0`, want: "-0"},
		{name: "negative decimal zero", raw: `-0.0`, want: "-0"},
		{name: "below threshold", raw: `999999`, want: "999999"},
		{name: "negative below threshold", raw: `-999999`, want: "-999999"},
		{name: "decimal below threshold", raw: `999999.0`, want: "999999"},
		{name: "negative decimal below threshold", raw: `-999999.0`, want: "-999999"},
		{name: "fraction", raw: `1.5`, want: "1.5"},
		{name: "negative fraction", raw: `-1.5`, want: "-1.5"},
		{name: "threshold", raw: `1000000`, want: "1000000"},
		{name: "negative threshold", raw: `-1000000`, want: "-1000000"},
		{name: "integer", raw: `1234567`, want: "1234567"},
		{name: "negative integer", raw: `-1234567`, want: "-1234567"},
		{name: "exponent", raw: `1e6`, want: "1000000"},
		{name: "signed uppercase exponent", raw: `1E+6`, want: "1000000"},
		{name: "negative exponent integer", raw: `-1e6`, want: "-1000000"},
		{name: "decimal integer", raw: `1000000.0`, want: "1000000"},
		{name: "negative decimal integer", raw: `-1000000.0`, want: "-1000000"},
		{name: "fraction rounds up", raw: `999999.99999999999`, want: "1e+06"},
		{name: "fraction rounds down", raw: `1000000.00000000001`, want: "1e+06"},
		{name: "negative fractional trap", raw: `-1000000.00000000001`, want: "-1e+06"},
		{name: "fractional exponent trap", raw: `100000000000000001e-11`, want: "1e+06"},
		{name: "exact large integer", raw: `9007199254740993`, want: "9007199254740993"},
		{name: "exact large decimal", raw: `9007199254740993.0`, want: "9007199254740993"},
		{name: "exact large exponent", raw: `90071992547409930e-1`, want: "9007199254740993"},
		{name: "negative exact large integer", raw: `-9007199254740993`, want: "-9007199254740993"},
		{name: "negative exact large decimal", raw: `-9007199254740993.0`, want: "-9007199254740993"},
		{name: "negative exact large exponent", raw: `-90071992547409930e-1`, want: "-9007199254740993"},
		{name: "beyond int64", raw: `99999999999999999999999`, want: "99999999999999999999999"},
		{name: "beyond int64 decimal", raw: `99999999999999999999999.0`, want: "99999999999999999999999"},
		{name: "beyond int64 exponent", raw: `999999999999999999999990e-1`, want: "99999999999999999999999"},
		{name: "negative beyond int64 decimal", raw: `-99999999999999999999999.0`, want: "-99999999999999999999999"},
		{name: "negative beyond int64 exponent", raw: `-999999999999999999999990e-1`, want: "-99999999999999999999999"},
		{name: "309 digit integer", raw: "1" + strings.Repeat("0", 308), want: power308},
		{name: "309 digit exponent", raw: `1e308`, want: power308},
		{name: "negative 309 digit exponent", raw: `-1e308`, want: "-" + power308},
		{name: "maximum finite 309 digits", raw: "17976931348623157" + strings.Repeat("0", 292), want: maxFiniteInteger},
		{name: "negative maximum finite 309 digits", raw: "-17976931348623157" + strings.Repeat("0", 292), want: "-" + maxFiniteInteger},
		{name: "310 digit overflow", raw: "1" + strings.Repeat("0", 309), wantErr: true},
		{name: "400 digit overflow", raw: strings.Repeat("9", 400), wantErr: true},
		{name: "exponent overflow 309", raw: `1e309`, wantErr: true},
		{name: "exponent overflow 400", raw: `1e400`, wantErr: true},
		{name: "huge exponent overflow", raw: `1e1000000000`, wantErr: true},
		{name: "negative overflow", raw: `-1e400`, wantErr: true},
		{name: "huge exponent underflow", raw: `1e-1000000000`, want: "0"},
		{name: "negative huge exponent underflow", raw: `-1e-1000000000`, want: "-0"},
		{name: "huge zero exponent", raw: `0e1000000000`, want: "0"},
		{name: "negative huge zero exponent", raw: `-0e1000000000`, want: "-0"},
		{name: "exponent outside int", raw: `0e9999999999999999999999999999999999999999`, want: "0"},
		{name: "negative zero exponent outside int", raw: `-0e9999999999999999999999999999999999999999`, want: "-0"},
		{name: "underflow exponent outside int", raw: `1e-9999999999999999999999999999999999999999`, want: "0"},
		{name: "fallback A equality boundary", raw: "1" + strings.Repeat("0", 100006) + "e-100000", want: "0"},
		{name: "fallback B digit cap", raw: "0." + strings.Repeat("0", 9999) + "1e100000", want: "1"},
		{name: "long token accepted", raw: "1000000." + strings.Repeat("0", 100006), want: "1000000"},
		{name: "long cancelling coefficient fallback", raw: "1000000" + strings.Repeat("0", 1000) + "e-1000", want: "1e-201"},
		{name: "text", raw: `"text"`, want: "text"},
		{name: "quoted exponent", raw: `"1e6"`, want: "1e6"},
		{name: "quoted decimal", raw: `"1000000.0"`, want: "1000000.0"},
		{name: "quoted large integer", raw: `"9007199254740993"`, want: "9007199254740993"},
		{name: "null", raw: `null`, want: "<nil>"},
		{name: "true", raw: `true`, want: "true"},
		{name: "false", raw: `false`, want: "false"},
		{name: "nested exponent", raw: `{"a":1e6}`, want: "map[a:1e+06]"},
		{name: "nested integer", raw: `{"a":1000000}`, want: "map[a:1e+06]"},
		{name: "nested array", raw: `[1.50]`, want: "[1.5]"},
		{name: "recursive mixed collection", raw: `{"a":[{"n":9007199254740993},1.50,true,null]}`, want: "map[a:[map[n:9.007199254740992e+15] 1.5 true <nil>]]"},
		{name: "empty object", raw: `{}`, want: "map[]"},
		{name: "empty array", raw: `[]`, want: "[]"},
		{name: "nested overflow", raw: `{"a":[1e400]}`, wantErr: true},
		{name: "leading zero invalid", raw: `01000000`, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Exercise the raw argument-map boundary even for text and null.
			var args map[string]interface{}
			err := DecodeJSON([]byte(`{"n":`+tt.raw+`}`), &args)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected legacy decode error")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got := FormatToolArgument(args["n"]); got != tt.want {
				t.Errorf("FormatToolArgument = %q, want %q", got, tt.want)
			}
		})
	}
	for _, tt := range []struct {
		name  string
		value any
		want  string
	}{
		{name: "existing int", value: int(1000000), want: "1000000"},
		{name: "existing float", value: float64(1000000), want: "1e+06"},
		{name: "existing nil map", value: map[string]interface{}(nil), want: "map[]"},
		{name: "existing nil slice", value: []interface{}(nil), want: "[]"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := FormatToolArgument(tt.value); got != tt.want {
				t.Errorf("FormatToolArgument = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestLargeIntegerArgs_DecodeJSON(t *testing.T) {
	tests := []struct {
		name    string
		raw     string
		want    map[string]interface{}
		wantErr bool
	}{
		{name: "number preserved", raw: `{"n":9007199254740993}`, want: map[string]interface{}{"n": json.Number("9007199254740993")}},
		{name: "empty", raw: `{}`, want: map[string]interface{}{}},
		{name: "null", raw: `null`},
		{name: "whitespace", raw: " {\"n\":1000000}\n\t ", want: map[string]interface{}{"n": json.Number("1000000")}},
		{name: "successful duplicate", raw: `{"n":999999,"n":1000000}`, want: map[string]interface{}{"n": json.Number("1000000")}},
		{name: "successful subtree replacement", raw: `{"n":{"ok":1},"n":1000000}`, want: map[string]interface{}{"n": json.Number("1000000")}},
		{name: "successful map replacement", raw: `{"n":{"old":1},"n":{"new":2}}`, want: map[string]interface{}{"n": map[string]interface{}{"new": json.Number("2")}}},
		{name: "malformed", raw: `{"n":`, wantErr: true},
		{name: "empty input", raw: ``, wantErr: true},
		{name: "trailing value", raw: `{"n":1} {}`, wantErr: true},
		{name: "trailing junk", raw: `{"n":1} junk`, wantErr: true},
		{name: "overflow", raw: `{"n":1e400}`, wantErr: true},
		{name: "400 digits", raw: `{"n":` + strings.Repeat("9", 400) + `}`, wantErr: true},
		{name: "nested overflow", raw: `{"n":[{"bad":1e400}]}`, wantErr: true},
		{name: "nested 400 digits", raw: `{"n":{"bad":` + strings.Repeat("9", 400) + `}}`, wantErr: true},
		{name: "overwritten scalar overflow", raw: `{"n":1e400,"n":1000000}`, wantErr: true},
		{name: "overwritten subtree overflow", raw: `{"n":{"bad":1e400},"n":1000000}`, wantErr: true},
		{name: "overwritten 400 digits", raw: `{"n":` + strings.Repeat("9", 400) + `,"n":1000000}`, wantErr: true},
		{name: "array mismatch", raw: `[]`, wantErr: true},
		{name: "zero mismatch", raw: `0`, wantErr: true},
		{name: "negative zero mismatch", raw: `-0`, wantErr: true},
		{name: "negative decimal zero mismatch", raw: `-0.0`, wantErr: true},
		{name: "boolean mismatch", raw: `true`, wantErr: true},
		{name: "string mismatch", raw: `"text"`, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got map[string]interface{}
			err := DecodeJSON([]byte(tt.raw), &got)
			if (err != nil) != tt.wantErr {
				t.Fatalf("DecodeJSON error = %v, want error %t", err, tt.wantErr)
			}
			if !tt.wantErr && !reflect.DeepEqual(got, tt.want) {
				t.Errorf("DecodeJSON = %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestLargeIntegerArgs_DecodeJSON_TypedDestination(t *testing.T) {
	type response struct {
		Count int                    `json:"count"`
		Cost  float64                `json:"cost"`
		Args  map[string]interface{} `json:"args"`
	}
	for _, tt := range []struct {
		name    string
		raw     string
		wantErr bool
	}{
		{name: "ignored scalar overflow", raw: `{"unknown":1e400}`},
		{name: "ignored nested overflow", raw: `{"unknown":{"bad":1e400}}`},
		{name: "ignored 400 digits", raw: `{"unknown":` + strings.Repeat("9", 400) + `}`},
		{name: "known float overflow", raw: `{"cost":1e400}`, wantErr: true},
		{name: "known integer overflow", raw: `{"count":1e400}`, wantErr: true},
		{name: "overwritten typed overflow", raw: `{"cost":1e400,"cost":1}`, wantErr: true},
		{name: "overwritten typed subtree", raw: `{"args":{"bad":1e400},"args":{"n":1000000}}`, wantErr: true},
		{name: "overwritten typed scalar", raw: `{"args":{"n":1e400,"n":1000000}}`, wantErr: true},
		{name: "successful typed duplicate", raw: `{"count":1,"count":2,"args":{"n":999999,"n":1000000}}`},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var got response
			err := DecodeJSON([]byte(tt.raw), &got)
			if (err != nil) != tt.wantErr {
				t.Fatalf("DecodeJSON error = %v, want error %t", err, tt.wantErr)
			}
		})
	}
	// A reused destination must not contaminate the fresh legacy validation.
	got := response{Count: 7, Args: map[string]interface{}{"n": "old"}}
	if err := DecodeJSON([]byte(`{"args":{"n":1000000}}`), &got); err != nil {
		t.Fatal(err)
	}
	if got.Count != 7 || got.Args["n"] != json.Number("1000000") {
		t.Fatalf("reused destination = %#v", got)
	}
	var pointer *response
	if err := DecodeJSON([]byte(`{"count":1}`), &pointer); err != nil || pointer == nil || pointer.Count != 1 {
		t.Fatalf("pointer destination = %#v, error = %v", pointer, err)
	}
}

func TestLargeIntegerArgs_OpenAI_DecodeErrors(t *testing.T) {
	// Only the outer string is encoded; numeric argument tokens stay raw.
	toolBody := func(arguments string) string {
		quoted, err := json.Marshal(arguments)
		if err != nil {
			t.Fatal(err)
		}
		return `{"model":"gpt-4o","choices":[{"message":{"tool_calls":[{"id":"call_1","type":"function","function":{"name":"example","arguments":` + string(quoted) + `}}]},"finish_reason":"tool_calls"}]}`
	}
	base := toolBody(`{"n":1000000}`)
	tests := []struct {
		name    string
		body    string
		wantErr bool
		want    map[string]string
	}{
		{name: "outer malformed", body: `{"choices":`, wantErr: true},
		{name: "outer trailing value", body: base + `{}`, wantErr: true},
		{name: "outer trailing junk", body: base + `junk`, wantErr: true},
		{name: "known integer overflow", body: `{"choices":[{}],"usage":{"prompt_tokens":1e400}}`, wantErr: true},
		{name: "known 400 digit overflow", body: `{"choices":[{}],"usage":{"completion_tokens":` + strings.Repeat("9", 400) + `}}`, wantErr: true},
		{name: "known nested overflow", body: `{"choices":[{}],"usage":{"prompt_tokens_details":{"cached_tokens":1e400}}}`, wantErr: true},
		{name: "overwritten known scalar", body: `{"choices":[{}],"usage":{"prompt_tokens":1e400,"prompt_tokens":1}}`, wantErr: true},
		{name: "overwritten known subtree", body: `{"choices":[{}],"usage":{"prompt_tokens":1e400},"usage":{"prompt_tokens":1}}`, wantErr: true},
		{name: "outer type mismatch", body: `{"choices":[{}],"usage":{"prompt_tokens":true}}`, wantErr: true},
		{name: "unknown scalar overflow", body: `{"unknown":1e400,` + base[1:], want: map[string]string{"n": "1000000"}},
		{name: "unknown subtree overflow", body: `{"unknown":{"bad":1e400},` + base[1:], want: map[string]string{"n": "1000000"}},
		{name: "unknown 400 digits", body: `{"unknown":` + strings.Repeat("9", 400) + `,` + base[1:], want: map[string]string{"n": "1000000"}},
		{name: "successful outer duplicate", body: `{"usage":{"prompt_tokens":1,"prompt_tokens":2},` + base[1:], want: map[string]string{"n": "1000000"}},
		{name: "successful outer subtree", body: `{"usage":{"prompt_tokens":1},"usage":{"completion_tokens":2},` + base[1:], want: map[string]string{"n": "1000000"}},
	}
	for _, tt := range []struct {
		name string
		raw  string
		want map[string]string
	}{
		{name: "inner malformed", raw: `{"n":`},
		{name: "inner trailing value", raw: `{"n":1} {}`},
		{name: "inner trailing junk", raw: `{"n":1} junk`},
		{name: "inner overflow", raw: `{"n":1e400}`},
		{name: "inner 400 digits", raw: `{"n":` + strings.Repeat("9", 400) + `}`},
		{name: "inner nested overflow", raw: `{"n":[{"bad":1e400}]}`},
		{name: "inner nested 400 digits", raw: `{"n":{"bad":` + strings.Repeat("9", 400) + `}}`},
		{name: "inner overwritten scalar overflow", raw: `{"n":1e400,"n":1000000}`},
		{name: "inner overwritten subtree overflow", raw: `{"n":{"bad":1e400},"n":1000000}`},
		{name: "inner successful scalar duplicate", raw: `{"n":999999,"n":1000000}`, want: map[string]string{"n": "1000000"}},
		{name: "inner successful subtree duplicate", raw: `{"n":{"ok":1},"n":1000000}`, want: map[string]string{"n": "1000000"}},
		{name: "inner successful map replacement", raw: `{"n":{"old":1},"n":{"new":2}}`, want: map[string]string{"n": "map[new:2]"}},
		{name: "inner empty", raw: `{}`},
		{name: "inner null", raw: `null`},
		{name: "inner array mismatch", raw: `[]`},
		{name: "inner scalar mismatch", raw: `0`},
		{name: "inner negative zero mismatch", raw: `-0`},
		{name: "inner decimal zero mismatch", raw: `-0.0`},
		{name: "inner boolean mismatch", raw: `true`},
		{name: "inner string mismatch", raw: `"text"`},
	} {
		tests = append(tests, struct {
			name    string
			body    string
			wantErr bool
			want    map[string]string
		}{name: tt.name, body: toolBody(tt.raw), want: tt.want})
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(tt.body))
			}))
			defer server.Close()
			o, err := NewOpenAI("test-key", WithOpenAIBaseURL(server.URL))
			if err != nil {
				t.Fatal(err)
			}
			resp, err := o.Send(context.Background(), &Request{Model: "gpt-4o"})
			if tt.wantErr {
				var providerErr *ProviderError
				if !errors.As(err, &providerErr) || providerErr.Category != ErrCategoryServer {
					t.Fatalf("Send error = %v, want server ProviderError", err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if len(resp.ToolCalls) != 1 {
				t.Fatalf("ToolCalls = %#v, want one call", resp.ToolCalls)
			}
			want := tt.want
			if want == nil {
				want = map[string]string{}
			}
			if !reflect.DeepEqual(resp.ToolCalls[0].Arguments, want) {
				t.Errorf("Arguments = %#v, want %#v", resp.ToolCalls[0].Arguments, want)
			}
		})
	}
}
