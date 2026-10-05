# TranslatorSage Go SDK

Official Go SDK for the [TranslatorSage API](https://developers.translatorsage.com) — TTS, STT, dubbing, translate, voices, agents, and more.

- **Idiomatic Go**: contexts, typed errors (`errors.As`), struct tags
- **Zero external dependencies** (stdlib only)
- **ElevenLabs drop-in** via `github.com/kokhp/tsage-sdk-go/tsage/elevenlabscompat`
- Automatic retry on 401/429 with exponential backoff

## Install

```sh
go get github.com/kokhp/tsage-sdk-go@latest
```

## Auth

```go
import tsage "github.com/kokhp/tsage-sdk-go/tsage"

client := tsage.NewClient(os.Getenv("TSAGE_API_KEY"))
// or override endpoint:
// client := tsage.NewClient("tsa_live_...", tsage.WithBaseURL("https://staging.translatorsage.com"))
```

## Quickstart

### Text to speech

```go
ctx := context.Background()
audio, err := client.TTS.SynthesizeBytes(ctx, tsage.TTSRequest{
    Text: "Hello from the TranslatorSage Go SDK.",
    VoiceID: "default",
    Lang: "en",
})
if err != nil { log.Fatal(err) }
_ = os.WriteFile("hello.mp3", audio, 0o644)
```

### Translate

```go
r, err := client.Translate.Text(ctx, tsage.TranslateRequest{
    Text: "The sage speaks.", TargetLang: "ja",
})
log.Println(r.TranslatedText)
```

### Dubbing

```go
job, err := client.Dubbing.Create(ctx, tsage.DubbingRequest{
    SourceURL:   "https://cdn.example.com/clip.mp4",
    TargetLang:  "es",
    NumSpeakers: 2,
})
done, err := client.Dubbing.Wait(ctx, job["job_id"].(string), 5*time.Second)
audio, err := client.Dubbing.GetAudio(ctx, done["job_id"].(string))
```

### ElevenLabs drop-in

```go
import ec "github.com/kokhp/tsage-sdk-go/tsage/elevenlabscompat"

client := ec.NewClient(os.Getenv("TSAGE_API_KEY"))
resp, _ := client.TextToSpeech.Convert(ctx, "default", ec.TTSConvertInput{
    Text: "Hi", ModelID: "tsage-tts-v1",
})
```

## Resource reference

| Namespace                | Methods                                                                  |
|--------------------------|--------------------------------------------------------------------------|
| `client.Consumers`       | `Signup`, `Me`, `ListAPIKeys`, `CreateAPIKey`, `RevokeAPIKey`, `Usage`   |
| `client.TTS`             | `Synthesize`, `SynthesizeBytes`                                          |
| `client.STT`             | `Transcribe`                                                             |
| `client.Translate`       | `Text`                                                                   |
| `client.Dubbing`         | `Create`, `Get`, `GetAudio`, `Wait`                                      |
| `client.Voices`          | `List`, `Add`, `PromoteToPVC`, `Delete`                                  |
| `client.SFX`             | `Generate`                                                               |
| `client.VoiceDesign`     | `Design`                                                                 |
| `client.VoiceChanger`    | `Convert`                                                                |
| `client.Dialogue`        | `Generate`                                                               |
| `client.AudioIsolation`  | `Isolate`                                                                |
| `client.Align`           | `Align`                                                                  |
| `client.Diarize`         | `Diarize`                                                                |
| `client.Agents`          | `Converse`                                                               |

## Error handling

```go
_, err := client.Consumers.Me(ctx)
var authErr *tsage.AuthError
switch {
case errors.As(err, &authErr):
    log.Printf("auth failure (status=%d, request_id=%s)", authErr.StatusCode, authErr.RequestID)
case errors.As(err, &tsage.RateLimitError{}):
    log.Printf("rate limited")
}
```

## Testing

```sh
go test ./...
```

Tests use `httptest.NewServer`, no network required.

## Reference

Full OpenAPI spec: [developers.translatorsage.com](https://developers.translatorsage.com)

## License

Apache-2.0
