// Package elevenlabscompat is a drop-in shim for the official ElevenLabs
// Go SDK (github.com/elevenlabs/elevenlabs-go). Swap the import and the
// API key — the method shapes line up.
//
//	import ec "github.com/kokhp/tsage-sdk-go/tsage/elevenlabscompat"
//
//	client := ec.NewClient("tsa_live_...")
//	audio, err := client.TextToSpeech.Convert(ctx, "default", ec.TTSConvertInput{Text: "Hi", ModelID: "tsage-tts-v1"})
package elevenlabscompat

import (
	"context"
	"errors"
	"strings"

	tsage "github.com/kokhp/tsage-sdk-go/tsage"
)

var modelIDToTier = map[string]string{
	"scribe_v1":       "standard",
	"scribe_standard": "standard",
	"scribe_live":     "live",
	"scribe_medical":  "medical",
	"scribe_raw":      "raw",
}

// Client is the ElevenLabs-shaped wrapper.
type Client struct {
	TextToSpeech *TextToSpeech
	SpeechToText *SpeechToText
	Voices       *Voices
	Dubbing      *Dubbing

	inner *tsage.Client
}

// NewClient wraps a tsage.Client under the ElevenLabs shape.
func NewClient(apiKey string, opts ...tsage.Option) *Client {
	inner := tsage.NewClient(apiKey, opts...)
	c := &Client{inner: inner}
	c.TextToSpeech = &TextToSpeech{inner: inner}
	c.SpeechToText = &SpeechToText{inner: inner}
	c.Voices = &Voices{inner: inner}
	c.Dubbing = &Dubbing{inner: inner}
	return c
}

// FromInner lets advanced users wrap an already-constructed tsage.Client.
func FromInner(inner *tsage.Client) *Client {
	return &Client{
		inner:        inner,
		TextToSpeech: &TextToSpeech{inner: inner},
		SpeechToText: &SpeechToText{inner: inner},
		Voices:       &Voices{inner: inner},
		Dubbing:      &Dubbing{inner: inner},
	}
}

type TTSConvertInput struct {
	Text          string
	ModelID       string
	VoiceSettings map[string]any
	OutputFormat  string
}

type TextToSpeech struct{ inner *tsage.Client }

func (t *TextToSpeech) Convert(ctx context.Context, voiceID string, in TTSConvertInput) (*tsage.TTSResponse, error) {
	return t.inner.TTS.Synthesize(ctx, tsage.TTSRequest{
		Text: in.Text, VoiceID: voiceID,
		ModelID: in.ModelID, VoiceSettings: in.VoiceSettings, OutputFormat: in.OutputFormat,
	})
}

type STTConvertInput struct {
	// File can be a URL string, local path string, []byte, or []byte via FileBytes.
	File         any
	FileBytes    []byte
	ModelID      string
	LanguageCode string
}

type SpeechToText struct{ inner *tsage.Client }

func (s *SpeechToText) Convert(ctx context.Context, in STTConvertInput) (map[string]any, error) {
	tier := modelIDToTier[in.ModelID]
	if tier == "" {
		tier = "standard"
	}
	req := tsage.STTRequest{Model: tier}
	if len(in.FileBytes) > 0 {
		req.Audio = in.FileBytes
	} else {
		switch f := in.File.(type) {
		case string:
			if strings.HasPrefix(strings.ToLower(f), "http://") || strings.HasPrefix(strings.ToLower(f), "https://") {
				req.AudioURL = f
			} else {
				req.AudioPath = f
			}
		case []byte:
			req.Audio = f
		default:
			return nil, errors.New("elevenlabscompat: SpeechToText.Convert requires File string (URL/path) or FileBytes")
		}
	}
	return s.inner.STT.Transcribe(ctx, req)
}

type Voices struct{ inner *tsage.Client }

type GetAllResponse struct {
	Voices []tsage.VoicePublic `json:"voices"`
}

func (v *Voices) GetAll(ctx context.Context) (*GetAllResponse, error) {
	voices, err := v.inner.Voices.List(ctx)
	if err != nil {
		return nil, err
	}
	return &GetAllResponse{Voices: voices}, nil
}

func (v *Voices) Search(ctx context.Context) (*GetAllResponse, error) { return v.GetAll(ctx) }

type AddVoiceInput struct {
	Name  string
	Files []string // require public URLs; local paths are not supported.
}

func (v *Voices) Add(ctx context.Context, in AddVoiceInput) (map[string]any, error) {
	if len(in.Files) == 0 {
		return nil, errors.New("elevenlabscompat: voices.Add requires at least one hosted audio URL")
	}
	first := in.Files[0]
	if !strings.HasPrefix(strings.ToLower(first), "http://") && !strings.HasPrefix(strings.ToLower(first), "https://") {
		return nil, errors.New("elevenlabscompat: TranslatorSage requires a hosted audio URL for voices.Add")
	}
	return v.inner.Voices.Add(ctx, tsage.VoiceAddRequest{Name: in.Name, ReferenceAudioURL: first})
}

func (v *Voices) Delete(ctx context.Context, voiceID string) (map[string]any, error) {
	return v.inner.Voices.Delete(ctx, voiceID)
}

type Dubbing struct{ inner *tsage.Client }

type DubbingCreateInput struct {
	SourceURL   string
	TargetLang  string
	Name        string
	NumSpeakers int
	Watermark   *bool
}

func (d *Dubbing) Create(ctx context.Context, in DubbingCreateInput) (map[string]any, error) {
	return d.inner.Dubbing.Create(ctx, tsage.DubbingRequest{
		SourceURL: in.SourceURL, TargetLang: in.TargetLang, Name: in.Name,
		NumSpeakers: in.NumSpeakers, Watermark: in.Watermark,
	})
}

func (d *Dubbing) Get(ctx context.Context, dubbingID string) (map[string]any, error) {
	return d.inner.Dubbing.Get(ctx, dubbingID)
}

func (d *Dubbing) GetAudio(ctx context.Context, dubbingID string) (map[string]any, error) {
	return d.inner.Dubbing.GetAudio(ctx, dubbingID)
}
