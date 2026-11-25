package runner

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/jrswab/axe/internal/config"
	"github.com/jrswab/axe/internal/provider"
	"github.com/jrswab/axe/internal/testutil"
	"github.com/jrswab/axe/internal/tool"
)

// Milestone 002 T2 (R1 integrated, R2, R3). Wire oracles use AWS's contracts, not Axe's request types:
// https://docs.aws.amazon.com/bedrock/latest/APIReference/API_runtime_SystemContentBlock.html
// https://docs.aws.amazon.com/bedrock/latest/APIReference/API_runtime_Tool.html
// https://docs.aws.amazon.com/bedrock/latest/APIReference/API_runtime_ToolConfiguration.html
// https://docs.aws.amazon.com/bedrock/latest/APIReference/API_runtime_ToolSpecification.html
// https://docs.aws.amazon.com/bedrock/latest/APIReference/API_runtime_ToolInputSchema.html
// This verifies local wire intent, not live acceptance or cache eligibility.
func TestBedrock_AWSRunParity(t *testing.T) {
	t.Run("setup", bedrockParitySetup)
	t.Run("static_and_task_edges", bedrockParityStaticAndTaskEdges)
	t.Run("independent_choices_and_outcome", bedrockParityDelegation)
	t.Run("usage_and_stream_fallback", bedrockParityUsage)
	t.Run("failure_boundaries", bedrockParityFailures)
}

const (
	parityReply    = `{"output":{"message":{"role":"assistant","content":[{"text":"Complete."}]}},"stopReason":"end_turn"`
	parityPoint    = `{"cachePoint":{"type":"default"}}`
	parityReadTool = `{"toolSpec":{"name":"read_file","description":"Read the contents of a file relative to the working directory. Returns line-numbered output in the format 'N: content' for each line.","inputSchema":{"json":{"type":"object","properties":{
		"path":{"type":"string","description":"Relative path to the file to read."},
		"offset":{"type":"string","description":"1-indexed line number to start reading from. Defaults to 1."},
		"limit":{"type":"string","description":"Maximum number of lines to return. Defaults to 2000."},
		"artifact":{"type":"string","description":"When \"true\", read from the artifact directory instead of the working directory."}},"required":["path"]}}}}`
	parityWriteTool = `{"toolSpec":{"name":"write_file","description":"Create or overwrite a file relative to the working directory. Creates parent directories as needed. Overwrites the file if it already exists.","inputSchema":{"json":{"type":"object","properties":{
		"path":{"type":"string","description":"Relative path to the file to write."},
		"content":{"type":"string","description":"The content to write to the file."},
		"artifact":{"type":"string","description":"When \"true\", write to the artifact directory; when \"false\", write to the working directory. If omitted, respects the agent-level default_write setting."}},"required":["path"]}}}}`
	parityCallTool = `{"toolSpec":{"name":"call_agent","description":"Delegate a task to a sub-agent. The sub-agent runs independently with its own context and returns only its final result. Available agents: parity-nested","inputSchema":{"json":{"type":"object","properties":{
		"agent":{"type":"string","description":"Name of the sub-agent to invoke (must be one of: parity-nested)"},
		"task":{"type":"string","description":"What you need the sub-agent to do"},
		"context":{"type":"string","description":"Additional context from your conversation to pass along"}},"required":["agent","task"]}}}}`
)

type parityCredentials struct{ access, secret, token string }

const parityRegion = "us-east-1"

var parityEnvCredentials = parityCredentials{"FAKEENVACCESS", "fixture-env-secret", "fixture-env-token"}

type parityFixture struct{ configDir, home, work string }

func newParityFixture(t *testing.T) parityFixture {
	t.Helper()
	cfg, _ := testutil.SetupXDGDirs(t)
	f := parityFixture{cfg, t.TempDir(), t.TempDir()}
	t.Chdir(f.work)
	t.Setenv("HOME", f.home)
	t.Setenv("USERPROFILE", f.home)
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	for _, key := range []string{"AWS_DEFAULT_REGION", "AXE_BEDROCK_REGION", "AWS_SHARED_CREDENTIALS_FILE", "AWS_PROFILE", "BEDROCK_API_KEY", "AXE_BEDROCK_BASE_URL", "AXE_ARTIFACT_DIR"} {
		t.Setenv(key, "")
	}
	t.Setenv("AWS_REGION", parityRegion)
	paritySetCredentials(t, parityEnvCredentials)
	return f
}

func paritySetCredentials(t *testing.T, creds parityCredentials) {
	t.Helper()
	t.Setenv("AWS_ACCESS_KEY_ID", creds.access)
	t.Setenv("AWS_SECRET_ACCESS_KEY", creds.secret)
	t.Setenv("AWS_SESSION_TOKEN", creds.token)
}

func parityWrite(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
}

func (f parityFixture) agent(t *testing.T, who, system, definition string) {
	t.Helper()
	parityWrite(t, filepath.Join(f.configDir, "agents", "parity-"+who+".toml"), fmt.Sprintf(`name = "parity-%s"
model = "bedrock/parity-%s"
workdir = %q
system_prompt = %q
max_turns = 2
%s`, who, who, f.work, system, definition))
}

func (f parityFixture) config(t *testing.T, region string, decoys bool) {
	t.Helper()
	definition := fmt.Sprintf("[providers.bedrock]\nregion = %q\n", region)
	if decoys {
		definition += "api_key = \"not-a-region-or-credential\"\nbase_url = \"https://config-override.invalid\"\n"
	}
	parityWrite(t, filepath.Join(f.configDir, "config.toml"), definition)
}

func parityMainOptions(stdout io.Writer) Options {
	return Options{AgentName: "parity-main", Prompt: "Main task.", Stdin: strings.NewReader(""), Stdout: stdout, Stderr: io.Discard}
}

func parityNested(t *testing.T, ctx context.Context, task, taskContext string) provider.ToolResult {
	t.Helper()
	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("load fixture config: %v", err)
	}
	return tool.ExecuteCallAgent(ctx, provider.ToolCall{
		ID: "direct-delegation", Name: "call_agent",
		Arguments: map[string]string{"agent": "parity-nested", "task": task, "context": taskContext},
	}, tool.ExecuteOptions{GlobalConfig: cfg, AllowedAgents: []string{"parity-nested"}, MaxDepth: 3, Stderr: io.Discard})
}

type parityTransport func(*http.Request) (*http.Response, error)

func (f parityTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

type parityWire map[string]json.RawMessage

type parityExchange struct {
	who  string
	wire parityWire
}

type parityReplyFunc func() (*http.Response, error)

func parityHTTP(status int, body string) parityReplyFunc {
	return func() (*http.Response, error) {
		return &http.Response{StatusCode: status, Header: http.Header{"Content-Type": {"application/json"}}, Body: io.NopCloser(strings.NewReader(body))}, nil
	}
}

// No underlying dialer or delegation to the previous transport: even an
// unexpected destination, redirect or retry cannot reach AWS (or any network).
func parityIntercept(t *testing.T, region string, creds parityCredentials, replies map[string][]parityReplyFunc) *[]parityExchange {
	t.Helper()
	var exchanges []parityExchange
	used := map[string]int{}
	previous := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = previous })
	http.DefaultTransport = parityTransport(func(r *http.Request) (*http.Response, error) {
		if r.Body != nil {
			defer func() { _ = r.Body.Close() }()
		}
		for who, script := range replies {
			endpoint := parityEndpoint(region, who)
			if r.URL.String() != endpoint || r.Method != http.MethodPost {
				continue
			}
			var wire parityWire
			body, err := io.ReadAll(r.Body)
			if err != nil || json.Unmarshal(body, &wire) != nil || wire == nil {
				t.Errorf("%s: expected a JSON request object (read error %v)", who, err)
				return nil, errors.New("invalid parity fixture request")
			}
			exchanges = append(exchanges, parityExchange{who, wire})
			parityVerifySignature(t, r, body, region, creds)
			i := used[who]
			used[who]++
			if i >= len(script) {
				t.Errorf("unexpected %s inference attempt %d", who, i+1)
				return nil, errors.New("unexpected parity fixture attempt")
			}
			return script[i]()
		}
		t.Errorf("unexpected outbound request blocked: %s %s", r.Method, r.URL)
		return nil, errors.New("outbound request blocked by parity fixture")
	})
	return &exchanges
}

func parityEndpoint(region, who string) string {
	return "https://bedrock-runtime." + region + ".amazonaws.com/model/parity-" + who + "/converse"
}

// Fixed canonical fields for this JSON POST, with no query or escaped path.
// Verify against the selected fixture secret, never Axe's signer or resolver.
// https://docs.aws.amazon.com/IAM/latest/UserGuide/reference_sigv-create-signed-request.html
func parityVerifySignature(t *testing.T, r *http.Request, body []byte, region string, creds parityCredentials) {
	t.Helper()
	stamp := r.Header.Get("X-Amz-Date")
	date, err := time.Parse("20060102T150405Z", stamp)
	if err != nil {
		t.Errorf("invalid X-Amz-Date %q", stamp)
		return
	}
	payload := fmt.Sprintf("%x", sha256.Sum256(body))
	if r.Host != r.URL.Host || r.Header.Get("Content-Type") != "application/json" || r.Header.Get("X-Amz-Content-Sha256") != payload || r.Header.Get("X-Amz-Security-Token") != creds.token || (creds.token == "" && len(r.Header.Values("X-Amz-Security-Token")) != 0) {
		t.Error("Converse host, JSON content type, payload hash or session-token header does not match")
	}
	canonical := fmt.Sprintf("content-type:application/json\nhost:%s\nx-amz-content-sha256:%s\nx-amz-date:%s\n", r.URL.Host, payload, stamp)
	signed := "content-type;host;x-amz-content-sha256;x-amz-date"
	if creds.token != "" {
		canonical += "x-amz-security-token:" + creds.token + "\n"
		signed += ";x-amz-security-token"
	}
	request := fmt.Sprintf("POST\n%s\n\n%s\n%s\n%s", r.URL.EscapedPath(), canonical, signed, payload)
	day := date.Format("20060102")
	scope := day + "/" + region + "/bedrock/aws4_request"
	toSign := fmt.Sprintf("AWS4-HMAC-SHA256\n%s\n%s\n%x", stamp, scope, sha256.Sum256([]byte(request)))
	key := []byte("AWS4" + creds.secret)
	for _, part := range []string{day, region, "bedrock", "aws4_request", toSign} {
		h := hmac.New(sha256.New, key)
		_, _ = io.WriteString(h, part)
		key = h.Sum(nil)
	}
	want := fmt.Sprintf("AWS4-HMAC-SHA256 Credential=%s/%s, SignedHeaders=%s, Signature=%x", creds.access, scope, signed, key)
	if r.Header.Get("Authorization") != want {
		t.Errorf("Converse signature does not match fixture access ID %s, region %s, and selected secret", creds.access, region)
	}
}

func parityAssertJSON(t *testing.T, label string, got json.RawMessage, want string) {
	t.Helper()
	if want == "" {
		if got != nil {
			t.Errorf("%s = %s, want absent", label, got)
		}
		return
	}
	var actual, expected any
	if json.Unmarshal([]byte(want), &expected) != nil {
		t.Fatalf("invalid %s oracle: %s", label, want)
	}
	if json.Unmarshal(got, &actual) != nil || !reflect.DeepEqual(actual, expected) {
		t.Errorf("%s = %s, want %s", label, got, want)
	}
}

func parityJSON(v any) string {
	// Used only with fixture strings and maps of strings (always JSON encodable).
	b, _ := json.Marshal(v)
	return string(b)
}

func parityText(role, content string) string {
	return fmt.Sprintf(`{"role":%q,"content":[{"text":%s}]}`, role, parityJSON(content))
}

func parityStatic(t *testing.T, got parityWire, system string, enabled bool, tools ...string) {
	t.Helper()
	var wantSystem, wantTools string
	if system != "" {
		wantSystem = `[{"text":` + parityJSON(system) + `}`
		if enabled {
			wantSystem += "," + parityPoint
		}
		wantSystem += "]"
	}
	if len(tools) != 0 {
		entries := strings.Join(tools, ",")
		if enabled {
			entries += "," + parityPoint
		}
		wantTools = `{"tools":[` + entries + `]}`
	}
	parityAssertJSON(t, "system", got["system"], wantSystem)
	parityAssertJSON(t, "toolConfig", got["toolConfig"], wantTools)
	parityAssertJSON(t, "no top-level cachePoint", got["cachePoint"], "")
	if bytes.Contains(got["messages"], []byte(`"cachePoint":`)) {
		t.Errorf("unexpected dynamic checkpoint: %s", got["messages"])
	}
}

func parityStable(t *testing.T, first, later parityWire, scope string) {
	t.Helper()
	for _, k := range []string{"system", "toolConfig"} {
		if !bytes.Equal(first[k], later[k]) {
			t.Errorf("serialized %s changed %s: %s -> %s", k, scope, first[k], later[k])
		}
	}
}

func parityCount(t *testing.T, exchanges *[]parityExchange, n int, detail any) {
	t.Helper()
	if len(*exchanges) != n {
		t.Fatalf("inference requests = %d, want %d; boundary result: %v", len(*exchanges), n, detail)
	}
}

func bedrockParitySetup(t *testing.T) {
	for _, who := range []string{"nested", "main"} {
		t.Run(who, func(t *testing.T) {
			for _, tt := range []struct {
				name, aws, standard, axe, configured, want string
			}{
				{"AWS_REGION", "us-east-1", "us-east-2", "us-west-1", "us-west-2", "us-east-1"},
				{"AWS_DEFAULT_REGION", "", "us-east-2", "us-west-1", "us-west-2", "us-east-2"},
				{"AXE_BEDROCK_REGION", "", "", "us-west-1", "us-west-2", "us-west-1"},
				{"provider_config", "", "", "", "us-west-2", "us-west-2"},
				{"missing_region", "", "", "", "", ""},
			} {
				t.Run(tt.name, func(t *testing.T) {
					f := newParityFixture(t)
					f.agent(t, who, "Static setup.", "")
					f.config(t, tt.configured, tt.want == "") // A decoy key must not rescue missing region.
					t.Setenv("AWS_REGION", tt.aws)
					t.Setenv("AWS_DEFAULT_REGION", tt.standard)
					t.Setenv("AXE_BEDROCK_REGION", tt.axe)
					if tt.want == "" {
						paritySetupError(t, who, []string{"region", "AWS_REGION", "config.toml"}, true, nil)
						return
					}
					paritySetupSuccess(t, who, tt.want, parityEnvCredentials, false)
				})
			}
			t.Run("malformed_resolved_region", func(t *testing.T) { bedrockParityMalformedRegion(t, who) })
			bedrockParityCredentials(t, who)
		})
	}
}

func bedrockParityMalformedRegion(t *testing.T, who string) {
	t.Helper()
	f := newParityFixture(t)
	f.agent(t, who, "Static setup.", "[retry]\nmax_retries = 0\n")
	f.config(t, "us-west-2", false)
	t.Setenv("AWS_REGION", "us_east_1") // An underscore is not valid in a DNS label.
	exchanges := parityIntercept(t, "", parityCredentials{}, nil)

	ctx := context.Background()
	var diagnostic string
	if who == "nested" {
		got := parityNested(t, ctx, "Nested task.", "")
		if !got.IsError || got.CallID != "direct-delegation" {
			t.Fatalf("Nested result = %+v, want error ToolResult", got)
		}
		diagnostic = got.Content
		parityNestedFailureNamesAgent(t, diagnostic)
	} else {
		got, err := Run(ctx, parityMainOptions(io.Discard))
		if got != nil || err == nil {
			t.Fatalf("Main Run = %+v, %v; want error and no result", got, err)
		}
		diagnostic = err.Error()
	}

	if msg := strings.ToLower(diagnostic); !strings.Contains(msg, "region") || !strings.Contains(msg, "invalid") {
		t.Errorf("setup diagnostic = %q; want an invalid-region diagnostic before inference", diagnostic)
	}
	parityCount(t, exchanges, 0, diagnostic)
}

func bedrockParityCredentials(t *testing.T, who string) {
	t.Helper()
	fileDefault := parityCredentials{"FAKEDEFAULTACCESS", "fixture-default-secret", ""}
	fileNamed := parityCredentials{"FAKENAMEDACCESS", "fixture-named-secret", "fixture-file-token"}
	fileHome := parityCredentials{"FAKEHOMEACCESS", "fixture-home-secret", ""}
	envWithoutToken := parityCredentials{parityEnvCredentials.access, parityEnvCredentials.secret, ""}
	const shared = `[default]
aws_access_key_id = FAKEDEFAULTACCESS
aws_secret_access_key = fixture-default-secret
[named]
aws_access_key_id = FAKENAMEDACCESS
aws_secret_access_key = fixture-named-secret
aws_session_token = fixture-file-token
[broken]
aws_access_key_id = FAKEBROKENACCESS
`
	const home = `[default]
aws_access_key_id = FAKEHOMEACCESS
aws_secret_access_key = fixture-home-secret
`
	for _, tt := range []struct {
		name, profile, file string
		env, want           parityCredentials
		decoys              bool
		why                 string
		cause               error
	}{
		{name: "complete_env_with_token_beats_file", profile: "named", file: "shared", env: parityEnvCredentials, want: parityEnvCredentials},
		{name: "complete_env_without_token_beats_file", profile: "named", file: "shared", env: envWithoutToken, want: envWithoutToken},
		{name: "access_only_falls_back_to_named_shared_profile", profile: "named", file: "shared", env: parityCredentials{access: "FAKEPARTIAL", token: "unused-env-token"}, want: fileNamed},
		{name: "secret_only_falls_back_to_shared_default_profile", file: "shared", env: parityCredentials{secret: "unused-partial-secret", token: "unused-env-token"}, want: fileDefault},
		{name: "isolated_home_default_path_and_profile", want: fileHome},
		{name: "ordinary_key_and_url_do_not_override_aws", file: "shared", want: fileDefault, decoys: true},
		{name: "missing_file", file: "missing", decoys: true, why: "does-not-exist.ini", cause: os.ErrNotExist},
		{name: "missing_profile", profile: "nonexistent", file: "shared", why: `profile "nonexistent" not found or missing credentials`},
		{name: "incomplete_profile", profile: "broken", file: "shared", why: `profile "broken" not found or missing credentials`},
	} {
		t.Run(tt.name, func(t *testing.T) {
			f := newParityFixture(t)
			f.agent(t, who, "Static setup.", "")
			f.config(t, "us-west-2", tt.decoys)
			paritySetCredentials(t, tt.env)
			t.Setenv("AWS_PROFILE", tt.profile)
			parityWrite(t, filepath.Join(f.home, ".aws", "credentials"), home)
			sharedPath := filepath.Join(f.home, "fixture-credentials.ini")
			parityWrite(t, sharedPath, shared)
			var missingPath string
			switch tt.file {
			case "shared":
				t.Setenv("AWS_SHARED_CREDENTIALS_FILE", sharedPath)
			case "missing":
				missingPath = filepath.Join(f.home, tt.why)
				t.Setenv("AWS_SHARED_CREDENTIALS_FILE", missingPath)
			}
			if tt.why != "" {
				guidance := []string{"AWS credentials not found", "AWS_ACCESS_KEY_ID", "AWS_SECRET_ACCESS_KEY", "credentials"}
				if who == "nested" && missingPath != "" {
					msg := paritySetupError(t, who, guidance, false, nil)
					for _, private := range []string{missingPath, filepath.Base(missingPath)} {
						if strings.Contains(msg, private) {
							t.Errorf("Nested credential diagnostic exposes local file information: %q", msg)
						}
					}
				} else {
					paritySetupError(t, who, append(guidance, tt.why), false, tt.cause)
				}
				return
			}
			paritySetupSuccess(t, who, parityRegion, tt.want, tt.decoys)
		})
	}
}

func paritySetupSuccess(t *testing.T, who, region string, creds parityCredentials, decoys bool) {
	t.Helper()
	exchanges := parityIntercept(t, region, creds, map[string][]parityReplyFunc{who: {parityHTTP(200, parityReply+`}`)}})
	ctx := context.Background()
	wantMessage := parityText("user", "Main task.")
	if who == "nested" {
		got := parityNested(t, ctx, "Nested task.", "")
		parityCount(t, exchanges, 1, got)
		if got != (provider.ToolResult{CallID: "direct-delegation", Content: "Complete."}) {
			t.Errorf("Nested outcome = %+v, want only final text", got)
		}
		wantMessage = parityText("user", "Task: Nested task.")
	} else {
		opts := parityMainOptions(io.Discard)
		if decoys {
			opts.APIKey, opts.BaseURL = "options-not-aws", "https://options-override.invalid"
		}
		got, err := Run(ctx, opts)
		parityCount(t, exchanges, 1, err)
		if err != nil || got == nil || got.Content != "Complete." {
			t.Errorf("Main Run = %+v, %v, want final text", got, err)
		}
	}
	wire := (*exchanges)[0].wire
	parityStatic(t, wire, "Static setup.", true)
	parityAssertJSON(t, "messages", wire["messages"], "["+wantMessage+"]")
}

// parityNestedFailureNamesAgent requires Nested setup failures to name the delegated sub-agent, not the provider.
func parityNestedFailureNamesAgent(t *testing.T, msg string) {
	t.Helper()
	if !strings.Contains(msg, `sub-agent "parity-nested" failed`) {
		t.Errorf("Nested setup failure = %q; want it to name sub-agent \"parity-nested\"", msg)
	}
}

func paritySetupError(t *testing.T, who string, parts []string, missingRegion bool, cause error) string {
	t.Helper()
	exchanges := parityIntercept(t, "", parityCredentials{}, nil)
	ctx := context.Background()
	var msg string
	if who == "nested" {
		got := parityNested(t, ctx, "Nested task.", "")
		if !got.IsError || got.CallID != "direct-delegation" {
			t.Fatalf("Nested setup result = %+v, want error ToolResult", got)
		}
		msg = got.Content
		parityNestedFailureNamesAgent(t, msg)
	} else {
		opts := parityMainOptions(io.Discard)
		opts.APIKey, opts.BaseURL = "options-not-aws", "https://options-override.invalid"
		result, err := Run(ctx, opts)
		if result != nil || err == nil {
			t.Fatalf("Main setup = %+v, %v; want setup error and no result", result, err)
		}
		if missingRegion {
			if !IsConfigError(err) {
				t.Errorf("missing region = %T %v, want ConfigError", err, err)
			}
		} else if !IsRuntimeError(err) || !strings.Contains(err.Error(), "failed to create provider") || errors.Unwrap(err) == nil || (cause != nil && !errors.Is(err, cause)) {
			t.Errorf("missing credentials = %T %v, want provider-creation RuntimeError with cause", err, err)
		}
		msg = err.Error()
	}
	if strings.Contains(msg, "API key for provider") {
		t.Errorf("AWS setup incorrectly used ordinary API-key lookup: %s", msg)
	}
	for _, part := range parts {
		if !strings.Contains(msg, part) {
			t.Errorf("setup diagnostic = %q; want %q", msg, part)
			break
		}
	}
	parityCount(t, exchanges, 0, msg)
	return msg
}

func bedrockParityStaticAndTaskEdges(t *testing.T) {
	for _, who := range []string{"main", "nested"} {
		for _, row := range []struct {
			name, system string
			tools        bool
		}{
			{"system_only", "Stable system.", false}, {"tools_only", "", true},
			{"system_and_tools", "Stable system.", true}, {"neither", "", false},
		} {
			for _, choice := range parityCacheChoices {
				t.Run(fmt.Sprintf("%s/%s/cache=%s", who, row.name, choice.name), func(t *testing.T) {
					f := newParityFixture(t)
					var definition string
					var schemas []string
					if row.tools {
						definition = "tools = [\"write_file\", \"read_file\"]\n"
						schemas = []string{parityWriteTool, parityReadTool}
					}
					f.agent(t, who, row.system, definition+choice.toml)
					exchanges := parityIntercept(t, parityRegion, parityEnvCredentials, map[string][]parityReplyFunc{who: {parityHTTP(200, parityReply+`}`)}})
					ctx := context.Background()
					var wantMessage string
					if who == "nested" {
						task := "Long dynamic task: " + strings.Repeat("keep this text; ", 300) + "last character!"
						got := parityNested(t, ctx, task, " \t\n")
						parityCount(t, exchanges, 1, got)
						if got != (provider.ToolResult{CallID: "direct-delegation", Content: "Complete."}) {
							t.Errorf("Nested outcome = %+v, want final text only", got)
						}
						wantMessage = parityText("user", "Task: "+task)
					} else {
						got, err := Run(ctx, parityMainOptions(io.Discard))
						parityCount(t, exchanges, 1, err)
						if err != nil || got == nil || got.Content != "Complete." {
							t.Fatalf("Main Run = %+v, %v", got, err)
						}
						wantMessage = parityText("user", "Main task.")
					}
					wire := (*exchanges)[0].wire
					parityStatic(t, wire, row.system, choice.enabled, schemas...)
					parityAssertJSON(t, "messages", wire["messages"], "["+wantMessage+"]")
				})
			}
		}
	}
	t.Run("empty_nested_task", func(t *testing.T) {
		f := newParityFixture(t)
		f.agent(t, "nested", "Stable system.", "")
		exchanges := parityIntercept(t, "", parityCredentials{}, nil)
		got := parityNested(t, context.Background(), "", "context is not a substitute for task")
		if got.CallID != "direct-delegation" || !got.IsError || !strings.Contains(got.Content, `"task" argument is required`) {
			t.Errorf("empty task = %+v, want actionable task error", got)
		}
		parityCount(t, exchanges, 0, got)
	})
}

type parityChoice struct {
	name, toml string
	enabled    bool
}

var parityCacheChoices = []parityChoice{
	{"omitted", "", true},
	{"true", "[params]\ncache_enabled = true\n", true},
	{"false", "[params]\ncache_enabled = false\n", false},
}

func bedrockParityDelegation(t *testing.T) {
	for _, main := range parityCacheChoices {
		for _, nested := range parityCacheChoices {
			t.Run(main.name+"_main/"+nested.name+"_nested", func(t *testing.T) {
				bedrockParityDelegatedRun(t, main, nested, "")
			})
		}
	}
	t.Run("follow_up_roles", func(t *testing.T) {
		main, nested := parityCacheChoices[0], parityCacheChoices[2]
		exchanges := bedrockParityDelegatedRun(t, main, nested, "")
		for _, followup := range exchanges[2:] {
			t.Run(followup.who, func(t *testing.T) {
				// Converse sends tool results back to the model in a user message.
				// https://docs.aws.amazon.com/bedrock/latest/userguide/tool-use-inference-call.html
				var messages []parityWire
				if err := json.Unmarshal(followup.wire["messages"], &messages); err != nil || len(messages) == 0 {
					t.Fatalf("missing follow-up messages: %s (%v)", followup.wire["messages"], err)
				}
				parityAssertJSON(t, "appended tool result role", messages[len(messages)-1]["role"], `"user"`)
			})
		}
	})
	t.Run("cross_run_static", func(t *testing.T) {
		// Each pair covers enabled and disabled static shapes, with the modes swapped between Main and Nested.
		for _, pair := range [][2]parityChoice{
			{parityCacheChoices[0], parityCacheChoices[2]},
			{parityCacheChoices[2], parityCacheChoices[1]},
		} {
			main, nested := pair[0], pair[1]
			t.Run("main_"+main.name+"/nested_"+nested.name, func(t *testing.T) {
				var first, second []parityExchange
				if !t.Run("first", func(t *testing.T) {
					first = bedrockParityDelegatedRun(t, main, nested, " / first run")
				}) || !t.Run("second", func(t *testing.T) {
					second = bedrockParityDelegatedRun(t, main, nested, " / second run")
				}) {
					return
				}
				for i, req := range first {
					if bytes.Equal(req.wire["messages"], second[i].wire["messages"]) {
						t.Errorf("%s request %d: conversation did not change across runs", req.who, i+1)
					}
					parityStable(t, req.wire, second[i].wire, fmt.Sprintf("across runs (%s request %d)", req.who, i+1))
				}
			})
		}
	})
}

func bedrockParityDelegatedRun(t *testing.T, main, nested parityChoice, dynamicSuffix string) []parityExchange {
	t.Helper()
	f := newParityFixture(t)
	f.agent(t, "main", "Main static.", "tools = [\"read_file\"]\nsub_agents = [\"parity-nested\"]\n"+main.toml)
	f.agent(t, "nested", "Nested static.", "tools = [\"write_file\", \"read_file\"]\n"+nested.toml)

	task := "Check local fixture: " + main.name + dynamicSuffix
	contextText := "Only this explicit context: " + nested.name + dynamicSuffix
	args := map[string]string{"agent": "parity-nested", "task": task, "context": contextText}
	delegate := `{"toolUse":{"toolUseId":"delegate-1","name":"call_agent","input":` + parityJSON(args) + `}}`
	const privateCall = `{"toolUse":{"toolUseId":"private-1","name":"write_file","input":{"path":"nested-private.txt","content":"private file body"}}}`
	const mainUsage1 = `{"inputTokens":3,"outputTokens":5,"cacheReadInputTokens":101,"cacheWriteInputTokens":103}`
	const mainUsage2 = `{"inputTokens":19,"outputTokens":23,"cacheReadInputTokens":131,"cacheWriteInputTokens":137}`
	const nestedUsage1 = `{"inputTokens":7,"outputTokens":11,"cacheReadInputTokens":107,"cacheWriteInputTokens":109}`
	const nestedUsage2 = `{"inputTokens":13,"outputTokens":17,"cacheReadInputTokens":113,"cacheWriteInputTokens":127}`
	exchanges := parityIntercept(t, parityRegion, parityEnvCredentials, map[string][]parityReplyFunc{
		"main": {
			parityHTTP(200, `{"output":{"message":{"role":"assistant","content":[{"text":"Delegating."},`+delegate+`]}},"stopReason":"tool_use","usage":`+mainUsage1+`}`),
			parityHTTP(200, `{"output":{"message":{"role":"assistant","content":[{"text":"Main finished."}]}},"stopReason":"end_turn","usage":`+mainUsage2+`}`),
		},
		"nested": {
			parityHTTP(200, `{"output":{"message":{"role":"assistant","content":[{"text":"Nested internal turn."},`+privateCall+`]}},"stopReason":"tool_use","usage":`+nestedUsage1+`}`),
			parityHTTP(200, `{"output":{"message":{"role":"assistant","content":[{"text":"Nested final text."}]}},"stopReason":"end_turn","usage":`+nestedUsage2+`}`),
		},
	})
	var stdout bytes.Buffer
	opts := parityMainOptions(&stdout)
	maxBudget := 100
	opts.JSON, opts.MaxTokens = true, &maxBudget
	opts.Messages = []Message{{Role: "user", Content: "Main earlier: " + nested.name + dynamicSuffix}, {Role: "assistant", Content: "Main earlier answer."}, {Role: "user", Content: "Main now: " + main.name + dynamicSuffix}}
	got, err := Run(context.Background(), opts)
	if err != nil || got == nil {
		t.Fatalf("Main delegation Run: %v", err)
	}
	parityCount(t, exchanges, 4, got.ToolCallDetails)
	wantOrder := []string{"main", "nested", "nested", "main"}
	for i, req := range *exchanges {
		if req.who != wantOrder[i] {
			t.Fatalf("request %d targets %s, want %s", i+1, req.who, wantOrder[i])
		}
	}
	m1, n1, n2, m2 := (*exchanges)[0].wire, (*exchanges)[1].wire, (*exchanges)[2].wire, (*exchanges)[3].wire
	for _, req := range []parityWire{m1, m2} {
		parityStatic(t, req, "Main static.", main.enabled, parityReadTool, parityCallTool)
	}
	for _, req := range []parityWire{n1, n2} {
		parityStatic(t, req, "Nested static.", nested.enabled, parityWriteTool, parityReadTool)
	}
	parityStable(t, m1, m2, "across Main turns")
	parityStable(t, n1, n2, "across Nested turns")

	mainHistory := []string{parityText("user", "Main earlier: "+nested.name+dynamicSuffix), parityText("assistant", "Main earlier answer."), parityText("user", "Main now: "+main.name+dynamicSuffix)}
	nestedTask := parityText("user", "Task: "+task+"\n\nContext:\n"+contextText)
	parityAssertJSON(t, "Main initial history", m1["messages"], "["+strings.Join(mainHistory, ",")+"]")
	parityAssertJSON(t, "Nested initial task/context", n1["messages"], "["+nestedTask+"]")
	parityHistoryWithToolResult(t, m2, append(mainHistory, `{"role":"assistant","content":[{"text":"Delegating."},`+delegate+`]}`),
		`[{"toolResult":{"toolUseId":"delegate-1","content":[{"text":"Nested final text."}],"status":"success"}}]`)
	parityHistoryWithToolResult(t, n2, []string{nestedTask, `{"role":"assistant","content":[{"text":"Nested internal turn."},` + privateCall + `]}`},
		`[{"toolResult":{"toolUseId":"private-1","content":[{"text":"wrote 17 bytes to nested-private.txt"}],"status":"success"}}]`)

	if got.Content != "Main finished." || got.ToolCalls != 1 || len(got.ToolCallDetails) != 1 || len(got.Artifacts) != 0 {
		t.Errorf("Main result/Outcome boundary = %+v", got)
	} else if d := got.ToolCallDetails[0]; d.Name != "call_agent" || d.IsError || d.Output != "Nested final text." {
		t.Errorf("expected only final Nested text, got %+v", d)
	}
	for _, private := range []string{"Nested internal turn.", "private-1", "nested-private.txt", "write_file", "private file body", "Nested static."} {
		if strings.Contains(stdout.String(), private) {
			t.Errorf("Nested internals crossed into Main JSON: %q", private)
		}
	}
	if got.InputTokens != 22 || got.OutputTokens != 28 || got.CacheReadTokens != 232 || got.CacheWriteTokens != 240 {
		t.Errorf("Main-only input/output/read/write = %d/%d/%d/%d, want 22/28/232/240", got.InputTokens, got.OutputTokens, got.CacheReadTokens, got.CacheWriteTokens)
	}
	if got.Budget != (BudgetState{Max: 100, Used: 98}) {
		t.Errorf("shared budget = %+v, want 98 of 100 (all input/output, no cache counters)", got.Budget)
	}
	return *exchanges
}

func parityHistoryWithToolResult(t *testing.T, req parityWire, prior []string, resultContent string) {
	t.Helper()
	var history []parityWire
	if err := json.Unmarshal(req["messages"], &history); err != nil || len(history) != len(prior)+1 {
		t.Fatalf("conversation does not append one tool result after history: %s", req["messages"])
	}
	var original []json.RawMessage
	if err := json.Unmarshal(req["messages"], &original); err != nil {
		t.Fatal(err)
	}
	for i, want := range prior {
		parityAssertJSON(t, fmt.Sprintf("history[%d]", i), original[i], want)
	}
	// The follow_up_roles subtests check the role separately from content and order.
	parityAssertJSON(t, "appended tool result content", history[len(prior)]["content"], resultContent)
}

func bedrockParityUsage(t *testing.T) {
	for _, tt := range []struct {
		name, usage                string
		input, output, read, write int
	}{
		{"omitted_usage", "", 0, 0, 0, 0},
		{"omitted_cache_counts", `,"usage":{"inputTokens":8,"outputTokens":3}`, 8, 3, 0, 0},
		{"zero_read_nonzero_write", `,"usage":{"inputTokens":8,"outputTokens":3,"cacheReadInputTokens":0,"cacheWriteInputTokens":11}`, 8, 3, 0, 11},
		{"nonzero_read_zero_write", `,"usage":{"inputTokens":8,"outputTokens":3,"cacheReadInputTokens":7,"cacheWriteInputTokens":0}`, 8, 3, 7, 0},
	} {
		t.Run(tt.name, func(t *testing.T) {
			f := newParityFixture(t)
			f.agent(t, "main", "Static stream fallback.", "")
			exchanges := parityIntercept(t, parityRegion, parityEnvCredentials, map[string][]parityReplyFunc{"main": {parityHTTP(200, parityReply+tt.usage+`}`)}})
			var stdout bytes.Buffer
			opts := parityMainOptions(&stdout)
			// cmd/run.go forwards --stream and --json through these options;
			// runner.Run emits the CLI envelope itself.
			opts.JSON, opts.Stream = true, true
			got, err := Run(context.Background(), opts)
			parityCount(t, exchanges, 1, err)
			if err != nil || got == nil {
				t.Fatalf("Run with --stream --json equivalent: %v", err)
			}
			if got.Content != "Complete." || got.InputTokens != tt.input || got.OutputTokens != tt.output || got.CacheReadTokens != tt.read || got.CacheWriteTokens != tt.write {
				t.Errorf("Run result = %+v; want Complete. and input/output/read/write %d/%d/%d/%d", got, tt.input, tt.output, tt.read, tt.write)
			}
			wire := (*exchanges)[0].wire
			parityStatic(t, wire, "Static stream fallback.", true)
			parityAssertJSON(t, "nonstreaming messages", wire["messages"], `[`+parityText("user", "Main task.")+`]`)
			resultJSON, err := json.Marshal(got)
			if err != nil {
				t.Fatal(err)
			}
			for name, raw := range map[string][]byte{"public Result JSON": resultJSON, "CLI --json envelope": stdout.Bytes()} {
				var envelope parityWire
				if err := json.Unmarshal(raw, &envelope); err != nil || envelope == nil {
					t.Fatalf("%s must contain exactly one JSON object: %q (%v)", name, raw, err)
				}
				parityAssertJSON(t, name+" content", envelope["content"], `"Complete."`)
				parityAssertJSON(t, name+" input_tokens", envelope["input_tokens"], fmt.Sprint(tt.input))
				parityAssertJSON(t, name+" output_tokens", envelope["output_tokens"], fmt.Sprint(tt.output))
				for key, n := range map[string]int{"cache_read_tokens": tt.read, "cache_write_tokens": tt.write} {
					want := ""
					if n != 0 {
						want = fmt.Sprint(n)
					}
					parityAssertJSON(t, name+" "+key, envelope[key], want)
				}
			}
		})
	}
}

type parityFailure struct {
	name, body, message, mainDiagnostic string
	status, mainAttempts                int
	category                            provider.ErrorCategory
	cause                               error
}

func bedrockParityFailures(t *testing.T) {
	lostResponse := parityFailure{name: "accepted_then_lost_response", category: provider.ErrCategoryServer, cause: io.EOF, mainDiagnostic: "provider server error: retry with backoff", mainAttempts: 3}
	rows := []parityFailure{
		{name: "authentication_401", status: 401, body: `{"message":"Missing signature"}`, category: provider.ErrCategoryAuth, message: "Missing signature", mainDiagnostic: "check credentials/token", mainAttempts: 1},
		{name: "authorization_403", status: 403, body: `{"message":"Access denied"}`, category: provider.ErrCategoryAuth, message: "Access denied", mainDiagnostic: "check credentials/token", mainAttempts: 1},
		{name: "generic_validation", status: 400, body: `{"type":"ValidationException","message":"Invalid request: check cache settings or model arguments"}`, category: provider.ErrCategoryBadRequest, message: "Invalid request: check cache settings or model arguments", mainDiagnostic: "bad request: validate request parameters", mainAttempts: 1},
		{name: "throttling", status: 429, body: `{"message":"Slow down"}`, category: provider.ErrCategoryRateLimit, message: "Slow down", mainDiagnostic: "backoff or increase quota", mainAttempts: 3},
		{name: "server", status: 500, body: `{"message":"Internal inference error"}`, category: provider.ErrCategoryServer, message: "Internal inference error", mainDiagnostic: "provider server error: retry with backoff", mainAttempts: 3},
		{name: "cancellation_after_request", category: provider.ErrCategoryTimeout, cause: context.Canceled, message: "context canceled", mainDiagnostic: "provider call failed", mainAttempts: 1},
		lostResponse,
		{name: "response_read", cause: io.ErrUnexpectedEOF, message: "failed to read response body: unexpected EOF", mainDiagnostic: "provider call failed", mainAttempts: 1},
		{name: "response_decode", body: `{"output":`, category: provider.ErrCategoryServer, message: "failed to parse response: unexpected end of JSON input", mainDiagnostic: "provider server error: retry with backoff", mainAttempts: 3},
	}
	for _, who := range []string{"main", "nested"} {
		for _, tt := range rows {
			t.Run(who+"/"+tt.name, func(t *testing.T) { bedrockParityFailure(t, who, tt) })
		}
	}
	// With retries disabled, an ambiguous loss gives exactly one attempt.
	t.Run("main/accepted_then_lost_without_retries", func(t *testing.T) {
		tt := lostResponse
		tt.mainAttempts = 1
		bedrockParityFailure(t, "main", tt)
	})
}

// A body failing after bytes have already arrived is distinct from no response
// and from a complete, malformed JSON response.
func parityBrokenBody() io.ReadCloser {
	return io.NopCloser(io.MultiReader(strings.NewReader(`{"output":`), parityReadError{}))
}

type parityReadError struct{}

func (parityReadError) Read([]byte) (int, error) { return 0, io.ErrUnexpectedEOF }

func bedrockParityFailure(t *testing.T, who string, tt parityFailure) {
	t.Helper()
	f := newParityFixture(t)
	retry := "[retry]\nmax_retries = 2\nbackoff = \"fixed\"\ninitial_delay_ms = 1\nmax_delay_ms = 1\n"
	if tt.name == "accepted_then_lost_response" && tt.mainAttempts == 1 {
		retry = "[retry]\nmax_retries = 0\n"
	}
	f.agent(t, who, "Static failure fixture.", "tools = [\"read_file\"]\n"+retry)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	reply := parityHTTP(tt.status, tt.body)
	switch tt.name {
	case "cancellation_after_request":
		reply = func() (*http.Response, error) {
			cancel() // The intercept has already consumed and verified the signed request.
			return nil, ctx.Err()
		}
	case "accepted_then_lost_response":
		reply = func() (*http.Response, error) { return nil, io.EOF }
		tt.message = fmt.Sprintf("Post %q: EOF", parityEndpoint(parityRegion, who))
	case "response_read":
		reply = func() (*http.Response, error) { return &http.Response{StatusCode: 200, Body: parityBrokenBody()}, nil }
	case "response_decode":
		reply = parityHTTP(200, tt.body)
	}
	attempts := 1
	if who == "main" {
		attempts = tt.mainAttempts
	}
	script := make([]parityReplyFunc, attempts)
	for i := range script {
		script[i] = reply
	}
	exchanges := parityIntercept(t, parityRegion, parityEnvCredentials, map[string][]parityReplyFunc{who: script})
	var wantUser string
	if who == "nested" {
		got := parityNested(t, ctx, "Failure task.", "")
		parityCount(t, exchanges, attempts, got)
		// Nested exposes the string of the cause (not a typed HTTP status or error chain).
		wantMessage := tt.message
		if tt.category != "" {
			wantMessage = string(tt.category) + ": " + wantMessage
		}
		want := provider.ToolResult{CallID: "direct-delegation", IsError: true, Content: `Error: sub-agent "parity-nested" failed - ` + wantMessage + `. You may retry or proceed without this result.`}
		if got != want {
			t.Errorf("Nested failure = %+v, want %+v", got, want)
		}
		wantUser = parityText("user", "Task: Failure task.")
	} else {
		var stdout bytes.Buffer
		got, err := Run(ctx, parityMainOptions(&stdout))
		parityCount(t, exchanges, attempts, err)
		if got != nil || stdout.Len() != 0 {
			t.Errorf("failed Main Run returned result or stdout: %+v / %q", got, stdout.String())
		}
		parityMainFailure(t, err, tt)
		wantUser = parityText("user", "Main task.")
	}
	for _, request := range *exchanges {
		parityStatic(t, request.wire, "Static failure fixture.", true, parityReadTool)
		parityAssertJSON(t, "unchanged messages on inference attempt", request.wire["messages"], "["+wantUser+"]")
		parityStable(t, (*exchanges)[0].wire, request.wire, "across attempts")
	}
}

func parityMainFailure(t *testing.T, err error, tt parityFailure) {
	t.Helper()
	if !IsRuntimeError(err) || !strings.Contains(err.Error(), tt.mainDiagnostic) || !strings.Contains(err.Error(), tt.message) {
		t.Fatalf("Main error = %T %v; want RuntimeError with %q and %q", err, err, tt.mainDiagnostic, tt.message)
	}
	if tt.cause != nil && !errors.Is(err, tt.cause) {
		t.Errorf("Main error %v lost cause %v", err, tt.cause)
	}
	category, typed := ProviderCategory(err)
	pe, _ := errors.AsType[*provider.ProviderError](err)
	if tt.name == "cancellation_after_request" || tt.category == "" {
		// Main's configured retry wrapper propagates cancellation directly;
		// an HTTP response-read failure is also not a typed provider error.
		if typed || pe != nil {
			t.Errorf("Main error unexpectedly has provider category %q: %v", category, err)
		}
		return
	}
	if !typed || pe == nil || category != tt.category || pe.Status != tt.status || pe.Message != tt.message {
		t.Fatalf("Main error = %v; provider error %+v (category %q), want %d %s %q", err, pe, category, tt.status, tt.category, tt.message)
	}
	switch tt.name {
	case "response_decode":
		if _, ok := errors.AsType[*json.SyntaxError](err); !ok {
			t.Errorf("Main decode error %v lost JSON syntax cause", err)
		}
	case "accepted_then_lost_response":
		if _, ok := errors.AsType[*url.Error](err); !ok {
			t.Errorf("Main transport error %v lost URL error cause", err)
		}
	default:
		if pe.Unwrap() != nil {
			t.Errorf("HTTP status error should not invent a cause: %v", pe.Unwrap())
		}
	}
}
