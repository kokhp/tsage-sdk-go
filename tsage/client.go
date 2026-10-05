package tsage

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"mime/multipart"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const (
	// DefaultBaseURL points at the production TranslatorSage orchestrator.
	DefaultBaseURL = "https://developers.translatorsage.com"
	// Version of this SDK.
	Version = "0.1.0-alpha"

	userAgent = "tsage-sdk-go/" + Version
)

// Client is the entry point for the SDK.
//
//	client := tsage.NewClient("tsa_live_...")
//	resp, err := client.TTS.Synthesize(ctx, tsage.TTSRequest{Text: "Hi", VoiceID: "default", Lang: "en"})
type Client struct {
	// configuration
	apiKey     string
	baseURL    string
	httpClient *http.Client
	maxRetries int

	// resource namespaces
	Consumers      *Consumers
	TTS            *TTS
	STT            *STT
	Translate      *Translate
	Dubbing        *Dubbing
	Voices         *Voices
	SFX            *SFX
	VoiceDesign    *VoiceDesign
	VoiceChanger   *VoiceChanger
	Dialogue       *Dialogue
	AudioIsolation *AudioIsolation
	Align          *Align
	Diarize        *Diarize
	Agents         *Agents
}

// Option configures a Client.
type Option func(*Client)

// WithBaseURL overrides the API endpoint.
func WithBaseURL(u string) Option { return func(c *Client) { c.baseURL = strings.TrimRight(u, "/") } }

// WithHTTPClient injects a custom http.Client (useful for proxies / instrumentation).
func WithHTTPClient(h *http.Client) Option { return func(c *Client) { c.httpClient = h } }

// WithMaxRetries sets the number of 401/429 retry attempts (default 3).
func WithMaxRetries(n int) Option { return func(c *Client) { c.maxRetries = n } }

// NewClient constructs a Client. apiKey can be empty for unauthenticated
// endpoints like /v1/consumers/signup.
func NewClient(apiKey string, opts ...Option) *Client {
	c := &Client{
		apiKey:     apiKey,
		baseURL:    DefaultBaseURL,
		httpClient: &http.Client{Timeout: 60 * time.Second},
		maxRetries: 3,
	}
	for _, o := range opts {
		o(c)
	}
	c.Consumers = &Consumers{c: c}
	c.TTS = &TTS{c: c}
	c.STT = &STT{c: c}
	c.Translate = &Translate{c: c}
	c.Dubbing = &Dubbing{c: c}
	c.Voices = &Voices{c: c}
	c.SFX = &SFX{c: c}
	c.VoiceDesign = &VoiceDesign{c: c}
	c.VoiceChanger = &VoiceChanger{c: c}
	c.Dialogue = &Dialogue{c: c}
	c.AudioIsolation = &AudioIsolation{c: c}
	c.Align = &Align{c: c}
	c.Diarize = &Diarize{c: c}
	c.Agents = &Agents{c: c}
	return c
}

// ---- Low-level request helpers (exposed for the compat package) ----

type requestOptions struct {
	path     string
	method   string
	query    url.Values
	jsonBody any
	// multipart file parts
	fileFields map[string]filePart
	// non-file multipart fields
	formFields map[string]string
	noAuth     bool
}

type filePart struct {
	Filename    string
	ContentType string
	Data        []byte
}

// Do runs a request and decodes the JSON response (or returns raw bytes if
// result is *[]byte). It handles auth, retry, and typed error mapping.
func (c *Client) Do(ctx context.Context, opts requestOptions, result any) error {
	u := c.baseURL + opts.path
	if len(opts.query) > 0 {
		q := url.Values{}
		for k, vs := range opts.query {
			for _, v := range vs {
				if v != "" {
					q.Add(k, v)
				}
			}
		}
		if encoded := q.Encode(); encoded != "" {
			u = u + "?" + encoded
		}
	}

	var attempts int
	for {
		attempts++
		req, body, err := c.buildRequest(ctx, opts.method, u, opts)
		if err != nil {
			return err
		}
		if !opts.noAuth {
			if c.apiKey == "" {
				return &AuthError{&APIError{Kind: "auth", Message: "no_api_key_configured", StatusCode: 401}}
			}
			req.Header.Set("Authorization", "Bearer "+c.apiKey)
		}
		req.Header.Set("User-Agent", userAgent)
		req.Header.Set("Accept", "application/json")
		if body != nil && req.Header.Get("Content-Type") == "" {
			req.Header.Set("Content-Type", "application/json")
		}

		resp, err := c.httpClient.Do(req)
		if err != nil {
			return &APIError{Kind: "transport", Message: err.Error()}
		}
		respBody, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()

		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			if result == nil {
				return nil
			}
			if raw, ok := result.(*[]byte); ok {
				*raw = respBody
				return nil
			}
			if len(respBody) == 0 {
				return nil
			}
			if err := json.Unmarshal(respBody, result); err != nil {
				return &APIError{Kind: "decode", Message: err.Error(), Body: string(respBody)}
			}
			return nil
		}

		if (resp.StatusCode == 401 || resp.StatusCode == 429) && attempts <= c.maxRetries {
			sleep := time.Duration(math.Pow(2, float64(attempts-1)) * float64(500*time.Millisecond))
			timer := time.NewTimer(sleep)
			select {
			case <-ctx.Done():
				timer.Stop()
				return ctx.Err()
			case <-timer.C:
			}
			continue
		}
		return errorForStatus(resp.StatusCode, respBody, resp.Header.Get("x-request-id"))
	}
}

func (c *Client) buildRequest(ctx context.Context, method, u string, opts requestOptions) (*http.Request, []byte, error) {
	// multipart branch
	if len(opts.fileFields) > 0 || len(opts.formFields) > 0 {
		var buf bytes.Buffer
		w := multipart.NewWriter(&buf)
		for k, v := range opts.formFields {
			if err := w.WriteField(k, v); err != nil {
				return nil, nil, err
			}
		}
		for k, fp := range opts.fileFields {
			part, err := w.CreateFormFile(k, fp.Filename)
			if err != nil {
				return nil, nil, err
			}
			if _, err := part.Write(fp.Data); err != nil {
				return nil, nil, err
			}
		}
		if err := w.Close(); err != nil {
			return nil, nil, err
		}
		req, err := http.NewRequestWithContext(ctx, method, u, &buf)
		if err != nil {
			return nil, nil, err
		}
		req.Header.Set("Content-Type", w.FormDataContentType())
		return req, buf.Bytes(), nil
	}

	// JSON body branch
	if opts.jsonBody != nil {
		pruned := prune(opts.jsonBody)
		b, err := json.Marshal(pruned)
		if err != nil {
			return nil, nil, err
		}
		req, err := http.NewRequestWithContext(ctx, method, u, bytes.NewReader(b))
		if err != nil {
			return nil, nil, err
		}
		return req, b, nil
	}

	req, err := http.NewRequestWithContext(ctx, method, u, nil)
	return req, nil, err
}

// prune drops nil values from maps to match the FastAPI "unset vs null" distinction.
func prune(v any) any {
	m, ok := v.(map[string]any)
	if !ok {
		return v
	}
	out := make(map[string]any, len(m))
	for k, vv := range m {
		if vv == nil {
			continue
		}
		out[k] = vv
	}
	return out
}

// Helper for building query values from scalar maps.
func queryFromMap(m map[string]any) url.Values {
	q := url.Values{}
	for k, v := range m {
		if v == nil {
			continue
		}
		switch x := v.(type) {
		case string:
			if x != "" {
				q.Set(k, x)
			}
		case int:
			q.Set(k, strconv.Itoa(x))
		case int64:
			q.Set(k, strconv.FormatInt(x, 10))
		case float64:
			q.Set(k, strconv.FormatFloat(x, 'f', -1, 64))
		case bool:
			q.Set(k, strconv.FormatBool(x))
		default:
			q.Set(k, fmt.Sprintf("%v", x))
		}
	}
	return q
}
