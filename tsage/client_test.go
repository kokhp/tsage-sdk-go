package tsage_test

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	tsage "github.com/kokhp/tsage-sdk-go/tsage"
	ec "github.com/kokhp/tsage-sdk-go/tsage/elevenlabscompat"
)

func TestTTSSynthesizeSendsBearerAndBody(t *testing.T) {
	var gotAuth, gotBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		if !strings.HasSuffix(r.URL.Path, "/v1/text-to-speech/default") {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		b, _ := io.ReadAll(r.Body)
		gotBody = string(b)
		w.Header().Set("Content-Type", "application/json")
		resp := map[string]any{
			"audio":                 base64.StdEncoding.EncodeToString([]byte("fake-audio-bytes")),
			"units_billed":          5,
			"cost_micros":           500,
			"balance_after_micros":  1_999_500,
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	client := tsage.NewClient("tsa_test_abc",
		tsage.WithBaseURL(srv.URL),
		tsage.WithMaxRetries(1),
	)
	resp, err := client.TTS.Synthesize(context.Background(), tsage.TTSRequest{
		Text: "Hello", VoiceID: "default", Lang: "en",
	})
	if err != nil {
		t.Fatalf("Synthesize: %v", err)
	}
	if resp.UnitsBilled != 5 {
		t.Errorf("units_billed = %d, want 5", resp.UnitsBilled)
	}
	audio, err := resp.AudioBytes()
	if err != nil || string(audio) != "fake-audio-bytes" {
		t.Errorf("AudioBytes = %q, %v", audio, err)
	}
	if gotAuth != "Bearer tsa_test_abc" {
		t.Errorf("auth = %q, want Bearer tsa_test_abc", gotAuth)
	}
	var decoded map[string]any
	_ = json.Unmarshal([]byte(gotBody), &decoded)
	if decoded["text"] != "Hello" {
		t.Errorf("body.text = %v", decoded["text"])
	}
	vs, _ := decoded["voice_settings"].(map[string]any)
	if vs["lang"] != "en" {
		t.Errorf("voice_settings.lang = %v", vs["lang"])
	}
}

func TestTranslate(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"translated_text": "salut", "target_lang": "fr"})
	}))
	defer srv.Close()
	client := tsage.NewClient("tsa_test_abc", tsage.WithBaseURL(srv.URL))
	resp, err := client.Translate.Text(context.Background(), tsage.TranslateRequest{Text: "hi", TargetLang: "fr"})
	if err != nil {
		t.Fatalf("Text: %v", err)
	}
	if resp.TranslatedText != "salut" {
		t.Errorf("got %q", resp.TranslatedText)
	}
}

func Test401RaisesAuthError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"detail":"invalid key"}`))
	}))
	defer srv.Close()
	client := tsage.NewClient("tsa_test_abc",
		tsage.WithBaseURL(srv.URL),
		tsage.WithMaxRetries(1),
		tsage.WithHTTPClient(&http.Client{Timeout: 2 * time.Second}),
	)
	_, err := client.Consumers.Me(context.Background())
	if err == nil {
		t.Fatal("expected auth error")
	}
	var ae *tsage.AuthError
	if !errors.As(err, &ae) {
		t.Errorf("err is not AuthError: %#v", err)
	}
}

func TestElevenLabsCompat(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"audio": "AA==", "units_billed": 1})
	}))
	defer srv.Close()
	client := ec.NewClient("tsa_test_abc", tsage.WithBaseURL(srv.URL))
	resp, err := client.TextToSpeech.Convert(context.Background(), "default", ec.TTSConvertInput{Text: "Hi", ModelID: "tsage-tts-v1"})
	if err != nil {
		t.Fatalf("Convert: %v", err)
	}
	if resp.UnitsBilled != 1 {
		t.Errorf("units_billed = %d", resp.UnitsBilled)
	}
}
