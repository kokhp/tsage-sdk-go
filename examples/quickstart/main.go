// Package main is a quickstart for the TranslatorSage Go SDK.
//
//	TSAGE_API_KEY=tsa_live_... go run ./examples/quickstart
package main

import (
	"context"
	"log"
	"os"

	tsage "github.com/kokhp/tsage-sdk-go/tsage"
)

func main() {
	ctx := context.Background()
	key := os.Getenv("TSAGE_API_KEY")
	if key == "" {
		log.Fatal("TSAGE_API_KEY must be set")
	}
	client := tsage.NewClient(key)

	// 1. Text to speech
	audio, err := client.TTS.SynthesizeBytes(ctx, tsage.TTSRequest{
		Text: "Hello from the TranslatorSage Go SDK.", VoiceID: "default", Lang: "en",
	})
	if err != nil {
		log.Fatalf("tts: %v", err)
	}
	if err := os.WriteFile("hello.mp3", audio, 0o644); err != nil {
		log.Fatalf("write: %v", err)
	}
	log.Printf("wrote hello.mp3 (%d bytes)", len(audio))

	// 2. Translation
	t, err := client.Translate.Text(ctx, tsage.TranslateRequest{Text: "The sage speaks.", TargetLang: "ja"})
	if err != nil {
		log.Fatalf("translate: %v", err)
	}
	log.Printf("translated: %s", t.TranslatedText)

	// 3. Voices
	voices, err := client.Voices.List(ctx)
	if err != nil {
		log.Fatalf("voices: %v", err)
	}
	log.Printf("voices available: %d", len(voices))
}
