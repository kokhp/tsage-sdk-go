package tsage

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

// ---------- Shared request/response types ----------

type TTSRequest struct {
	Text          string         `json:"text"`
	VoiceID       string         `json:"-"` // path param
	ModelID       string         `json:"model_id,omitempty"`
	VoiceSettings map[string]any `json:"voice_settings,omitempty"`
	OutputFormat  string         `json:"output_format,omitempty"`
	// Lang is a convenience that folds into VoiceSettings["lang"].
	Lang string `json:"-"`
}

type TTSResponse struct {
	Audio               string `json:"audio"`
	UnitsBilled         int64  `json:"units_billed"`
	CostMicros          int64  `json:"cost_micros"`
	BalanceAfterMicros  int64  `json:"balance_after_micros"`
	Model               string `json:"model,omitempty"`
}

// AudioBytes decodes the base64 audio field into raw bytes.
func (r *TTSResponse) AudioBytes() ([]byte, error) {
	if r.Audio == "" {
		return nil, errors.New("TTS response had no 'audio' field")
	}
	return base64.StdEncoding.DecodeString(r.Audio)
}

type TranslateRequest struct {
	Text       string `json:"text"`
	TargetLang string `json:"target_lang"`
	SourceLang string `json:"source_lang,omitempty"`
}

type TranslateResponse struct {
	TranslatedText string `json:"translated_text"`
	TargetLang     string `json:"target_lang,omitempty"`
	SourceLang     string `json:"source_lang,omitempty"`
}

type DubbingRequest struct {
	SourceURL   string `json:"source_url,omitempty"`
	SourceLang  string `json:"source_lang,omitempty"`
	TargetLang  string `json:"target_lang"`
	NumSpeakers int    `json:"num_speakers,omitempty"`
	Watermark   *bool  `json:"watermark,omitempty"`
	Name        string `json:"name,omitempty"`
}

type DubbingJob struct {
	JobID      string         `json:"job_id"`
	Status     string         `json:"status"`
	TargetLang string         `json:"target_lang,omitempty"`
	Extra      map[string]any `json:"-"`
}

type SignupRequest struct {
	Email             string `json:"email"`
	ReferralCode      string `json:"referral_code,omitempty"`
	SignupIP          string `json:"signup_ip,omitempty"`
	DeviceFingerprint string `json:"device_fingerprint,omitempty"`
}

type VoicePublic struct {
	VoiceID string `json:"voice_id"`
	Name    string `json:"name"`
}

type ConsumerProfile = map[string]any
type APIKeyPublic = map[string]any
type CreateAPIKeyResponse = map[string]any
type Usage = map[string]any
type ConverseResponse = map[string]any

// ---------- Namespaces ----------

type Consumers struct{ c *Client }

func (r *Consumers) Signup(ctx context.Context, req SignupRequest) (map[string]any, error) {
	var out map[string]any
	err := r.c.Do(ctx, requestOptions{
		method: http.MethodPost, path: "/v1/consumers/signup", noAuth: true,
		jsonBody: map[string]any{
			"email": req.Email, "referral_code": req.ReferralCode,
			"signup_ip": req.SignupIP, "device_fingerprint": req.DeviceFingerprint,
		},
	}, &out)
	return out, err
}

func (r *Consumers) Me(ctx context.Context) (ConsumerProfile, error) {
	var out ConsumerProfile
	err := r.c.Do(ctx, requestOptions{method: http.MethodGet, path: "/v1/consumers/me"}, &out)
	return out, err
}

func (r *Consumers) ListAPIKeys(ctx context.Context) ([]APIKeyPublic, error) {
	var out []APIKeyPublic
	err := r.c.Do(ctx, requestOptions{method: http.MethodGet, path: "/v1/consumers/me/api-keys"}, &out)
	return out, err
}

type CreateAPIKeyRequest struct {
	Name            string `json:"name,omitempty"`
	ConcurrentLimit *int   `json:"concurrent_limit,omitempty"`
}

func (r *Consumers) CreateAPIKey(ctx context.Context, req CreateAPIKeyRequest) (CreateAPIKeyResponse, error) {
	var out CreateAPIKeyResponse
	err := r.c.Do(ctx, requestOptions{
		method: http.MethodPost, path: "/v1/consumers/me/api-keys",
		jsonBody: map[string]any{"name": req.Name, "concurrent_limit": req.ConcurrentLimit},
	}, &out)
	return out, err
}

func (r *Consumers) RevokeAPIKey(ctx context.Context, apiKeyID string) (map[string]any, error) {
	var out map[string]any
	err := r.c.Do(ctx, requestOptions{method: http.MethodDelete, path: "/v1/consumers/me/api-keys/" + apiKeyID}, &out)
	return out, err
}

func (r *Consumers) Usage(ctx context.Context) (Usage, error) {
	var out Usage
	err := r.c.Do(ctx, requestOptions{method: http.MethodGet, path: "/v1/consumers/me/usage"}, &out)
	return out, err
}

type TTS struct{ c *Client }

func (r *TTS) Synthesize(ctx context.Context, req TTSRequest) (*TTSResponse, error) {
	if req.VoiceID == "" {
		req.VoiceID = "default"
	}
	if req.ModelID == "" {
		req.ModelID = "tsage-tts-v1"
	}
	if req.OutputFormat == "" {
		req.OutputFormat = "mp3_44100_128"
	}
	if req.Lang != "" {
		if req.VoiceSettings == nil {
			req.VoiceSettings = map[string]any{}
		}
		req.VoiceSettings["lang"] = req.Lang
	}
	out := &TTSResponse{}
	body := map[string]any{
		"text": req.Text, "model_id": req.ModelID,
		"voice_settings": req.VoiceSettings, "output_format": req.OutputFormat,
	}
	err := r.c.Do(ctx, requestOptions{
		method: http.MethodPost, path: "/v1/text-to-speech/" + req.VoiceID, jsonBody: body,
	}, out)
	return out, err
}

// SynthesizeBytes is a convenience wrapper that returns decoded audio bytes.
func (r *TTS) SynthesizeBytes(ctx context.Context, req TTSRequest) ([]byte, error) {
	resp, err := r.Synthesize(ctx, req)
	if err != nil {
		return nil, err
	}
	return resp.AudioBytes()
}

type STT struct{ c *Client }

type STTRequest struct {
	// Supply Audio OR AudioURL OR AudioPath.
	Audio               []byte
	AudioURL            string
	AudioPath           string
	Model               string  // "standard" / "live" / "medical" / "raw"
	DurationHintSeconds float64 // optional pricing-tier hint
	Filename            string
}

func (r *STT) Transcribe(ctx context.Context, req STTRequest) (map[string]any, error) {
	bytes_, filename, err := resolveAudio(req.Audio, req.AudioURL, req.AudioPath, req.Filename)
	if err != nil {
		return nil, err
	}
	model := req.Model
	if model == "" {
		model = "standard"
	}
	query := queryFromMap(map[string]any{
		"model":                   model,
		"duration_hint_seconds":   req.DurationHintSeconds,
	})
	// strip zero duration hint:
	if req.DurationHintSeconds == 0 {
		query.Del("duration_hint_seconds")
	}
	var out map[string]any
	err = r.c.Do(ctx, requestOptions{
		method: http.MethodPost, path: "/v1/speech-to-text", query: query,
		fileFields: map[string]filePart{"audio": {Filename: filename, ContentType: "application/octet-stream", Data: bytes_}},
	}, &out)
	return out, err
}

type Translate struct{ c *Client }

func (r *Translate) Text(ctx context.Context, req TranslateRequest) (*TranslateResponse, error) {
	out := &TranslateResponse{}
	err := r.c.Do(ctx, requestOptions{
		method: http.MethodPost, path: "/v1/translate",
		jsonBody: map[string]any{"text": req.Text, "target_lang": req.TargetLang, "source_lang": req.SourceLang},
	}, out)
	return out, err
}

type Dubbing struct{ c *Client }

func (r *Dubbing) Create(ctx context.Context, req DubbingRequest) (map[string]any, error) {
	var out map[string]any
	body := map[string]any{
		"source_url": req.SourceURL, "source_lang": req.SourceLang,
		"target_lang": req.TargetLang, "num_speakers": req.NumSpeakers,
		"name": req.Name,
	}
	if req.Watermark != nil {
		body["watermark"] = *req.Watermark
	}
	err := r.c.Do(ctx, requestOptions{method: http.MethodPost, path: "/v1/dubbing", jsonBody: body}, &out)
	return out, err
}

func (r *Dubbing) Get(ctx context.Context, jobID string) (map[string]any, error) {
	var out map[string]any
	err := r.c.Do(ctx, requestOptions{method: http.MethodGet, path: "/v1/dubbing/" + jobID}, &out)
	return out, err
}

func (r *Dubbing) GetAudio(ctx context.Context, jobID string) (map[string]any, error) {
	var out map[string]any
	err := r.c.Do(ctx, requestOptions{method: http.MethodGet, path: "/v1/dubbing/" + jobID + "/audio"}, &out)
	return out, err
}

// Wait polls until the job terminates (succeeded|failed) or ctx is cancelled.
func (r *Dubbing) Wait(ctx context.Context, jobID string, pollInterval time.Duration) (map[string]any, error) {
	if pollInterval == 0 {
		pollInterval = 5 * time.Second
	}
	for {
		job, err := r.Get(ctx, jobID)
		if err != nil {
			return nil, err
		}
		if s, _ := job["status"].(string); s == "succeeded" || s == "failed" {
			return job, nil
		}
		t := time.NewTimer(pollInterval)
		select {
		case <-ctx.Done():
			t.Stop()
			return nil, ctx.Err()
		case <-t.C:
		}
	}
}

type Voices struct{ c *Client }

func (r *Voices) List(ctx context.Context) ([]VoicePublic, error) {
	var out []VoicePublic
	err := r.c.Do(ctx, requestOptions{method: http.MethodGet, path: "/v1/voices"}, &out)
	return out, err
}

type VoiceAddRequest struct {
	Name              string   `json:"name"`
	ReferenceAudioURL string   `json:"reference_audio_url"`
	LangSupport       []string `json:"lang_support,omitempty"`
	ConsentDocURL     string   `json:"consent_doc_url,omitempty"`
}

func (r *Voices) Add(ctx context.Context, req VoiceAddRequest) (map[string]any, error) {
	var out map[string]any
	err := r.c.Do(ctx, requestOptions{method: http.MethodPost, path: "/v1/voices/add", jsonBody: map[string]any{
		"name": req.Name, "reference_audio_url": req.ReferenceAudioURL,
		"lang_support": req.LangSupport, "consent_doc_url": req.ConsentDocURL,
	}}, &out)
	return out, err
}

func (r *Voices) PromoteToPVC(ctx context.Context, voiceID string) (map[string]any, error) {
	var out map[string]any
	err := r.c.Do(ctx, requestOptions{method: http.MethodPost, path: "/v1/voices/" + voiceID + "/professional"}, &out)
	return out, err
}

func (r *Voices) Delete(ctx context.Context, voiceID string) (map[string]any, error) {
	var out map[string]any
	err := r.c.Do(ctx, requestOptions{method: http.MethodDelete, path: "/v1/voices/" + voiceID}, &out)
	return out, err
}

type SFX struct{ c *Client }

type SFXRequest struct {
	Text            string   `json:"text"`
	DurationSeconds *float64 `json:"duration_seconds,omitempty"`
	PromptInfluence *float64 `json:"prompt_influence,omitempty"`
}

func (r *SFX) Generate(ctx context.Context, req SFXRequest) (map[string]any, error) {
	var out map[string]any
	err := r.c.Do(ctx, requestOptions{method: http.MethodPost, path: "/v1/sound-effects", jsonBody: map[string]any{
		"text": req.Text, "duration_seconds": req.DurationSeconds, "prompt_influence": req.PromptInfluence,
	}}, &out)
	return out, err
}

type VoiceDesign struct{ c *Client }

type VoiceDesignRequest struct {
	Description string `json:"description"`
	Text        string `json:"text,omitempty"`
	Gender      string `json:"gender,omitempty"`
	Age         string `json:"age,omitempty"`
	Accent      string `json:"accent,omitempty"`
}

func (r *VoiceDesign) Design(ctx context.Context, req VoiceDesignRequest) (map[string]any, error) {
	var out map[string]any
	err := r.c.Do(ctx, requestOptions{method: http.MethodPost, path: "/v1/voice-design", jsonBody: map[string]any{
		"description": req.Description, "text": req.Text,
		"gender": req.Gender, "age": req.Age, "accent": req.Accent,
	}}, &out)
	return out, err
}

type VoiceChanger struct{ c *Client }

func (r *VoiceChanger) Convert(ctx context.Context, audio []byte, targetVoiceID, filename string) (map[string]any, error) {
	if filename == "" {
		filename = "input.wav"
	}
	var out map[string]any
	err := r.c.Do(ctx, requestOptions{
		method: http.MethodPost, path: "/v1/voice-changer",
		query: queryFromMap(map[string]any{"target_voice_id": targetVoiceID}),
		fileFields: map[string]filePart{"audio": {Filename: filename, ContentType: "application/octet-stream", Data: audio}},
	}, &out)
	return out, err
}

type Dialogue struct{ c *Client }

type DialogueTurn struct {
	Speaker string `json:"speaker"`
	Text    string `json:"text"`
}

func (r *Dialogue) Generate(ctx context.Context, turns []DialogueTurn, outputFormat string) (map[string]any, error) {
	if outputFormat == "" {
		outputFormat = "mp3_44100_128"
	}
	var out map[string]any
	err := r.c.Do(ctx, requestOptions{method: http.MethodPost, path: "/v1/dialogue", jsonBody: map[string]any{
		"turns": turns, "output_format": outputFormat,
	}}, &out)
	return out, err
}

type AudioIsolation struct{ c *Client }

func (r *AudioIsolation) Isolate(ctx context.Context, audio []byte, filename string) (map[string]any, error) {
	if filename == "" {
		filename = "audio.wav"
	}
	var out map[string]any
	err := r.c.Do(ctx, requestOptions{
		method: http.MethodPost, path: "/v1/audio-isolation",
		fileFields: map[string]filePart{"audio": {Filename: filename, ContentType: "application/octet-stream", Data: audio}},
	}, &out)
	return out, err
}

type Align struct{ c *Client }

func (r *Align) Align(ctx context.Context, audio []byte, transcript, filename string) (map[string]any, error) {
	if filename == "" {
		filename = "audio.wav"
	}
	var out map[string]any
	err := r.c.Do(ctx, requestOptions{
		method: http.MethodPost, path: "/v1/align",
		query:      queryFromMap(map[string]any{"transcript": transcript}),
		fileFields: map[string]filePart{"audio": {Filename: filename, ContentType: "application/octet-stream", Data: audio}},
	}, &out)
	return out, err
}

type Diarize struct{ c *Client }

func (r *Diarize) Diarize(ctx context.Context, audio []byte, numSpeakers int, filename string) (map[string]any, error) {
	if filename == "" {
		filename = "audio.wav"
	}
	q := queryFromMap(map[string]any{})
	if numSpeakers > 0 {
		q.Set("num_speakers", fmt.Sprintf("%d", numSpeakers))
	}
	var out map[string]any
	err := r.c.Do(ctx, requestOptions{
		method: http.MethodPost, path: "/v1/diarize", query: q,
		fileFields: map[string]filePart{"audio": {Filename: filename, ContentType: "application/octet-stream", Data: audio}},
	}, &out)
	return out, err
}

type Agents struct{ c *Client }

type ConverseRequest struct {
	Message string         `json:"message"`
	Hints   map[string]any `json:"hints,omitempty"`
}

func (r *Agents) Converse(ctx context.Context, agentID string, req ConverseRequest) (map[string]any, error) {
	var out map[string]any
	err := r.c.Do(ctx, requestOptions{method: http.MethodPost, path: "/v1/agents/" + agentID + "/converse", jsonBody: map[string]any{
		"message": req.Message, "hints": req.Hints,
	}}, &out)
	return out, err
}

// ---------- helpers ----------

func resolveAudio(data []byte, url, path, filename string) ([]byte, string, error) {
	if len(data) > 0 {
		if filename == "" {
			filename = "audio.wav"
		}
		return data, filename, nil
	}
	if path != "" {
		b, err := os.ReadFile(path)
		if err != nil {
			return nil, "", err
		}
		if filename == "" {
			filename = filepath.Base(path)
		}
		return b, filename, nil
	}
	if url != "" {
		resp, err := http.Get(url)
		if err != nil {
			return nil, "", err
		}
		defer resp.Body.Close()
		if resp.StatusCode >= 300 {
			return nil, "", fmt.Errorf("could not fetch %s: %s", url, resp.Status)
		}
		b, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, "", err
		}
		if filename == "" {
			filename = filepath.Base(resp.Request.URL.Path)
			if filename == "" || filename == "/" {
				filename = "audio.wav"
			}
		}
		return b, filename, nil
	}
	return nil, "", errors.New("STT requires Audio, AudioURL, or AudioPath")
}
