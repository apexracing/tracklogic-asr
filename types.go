package voice

import "github.com/apexracing/tracklogic-voice/assets"

// Language is a language prompt accepted by SenseVoice.
type Language string

const (
	LanguageAuto      Language = "auto"
	LanguageChinese   Language = "zh"
	LanguageEnglish   Language = "en"
	LanguageCantonese Language = "yue"
	LanguageJapanese  Language = "ja"
	LanguageKorean    Language = "ko"
	LanguageNoSpeech  Language = "nospeech"
)

// RecognizerConfig controls recognizer resources and ONNX CPU execution.
type RecognizerConfig struct {
	Assets     assets.ASRConfig
	NumThreads int
}

// TranscriptionOptions controls one transcription request.
type TranscriptionOptions struct {
	Language   Language
	WithoutITN bool
}

// TranscriptionResult contains recognized text and SenseVoice metadata.
type TranscriptionResult struct {
	Text     string
	Language string
	Emotion  string
	Events   []string
	Tokens   []string
	TokenIDs []int
}

// Config is retained for source compatibility.
// Deprecated: use RecognizerConfig.
type Config = RecognizerConfig

// Options is retained for source compatibility.
// Deprecated: use TranscriptionOptions.
type Options = TranscriptionOptions

// Result is retained for source compatibility.
// Deprecated: use TranscriptionResult.
type Result = TranscriptionResult

// SynthesizerConfig controls Kokoro resources and ONNX CPU execution.
type SynthesizerConfig struct {
	Assets     assets.TTSConfig
	NumThreads int
}

// SynthesisOptions controls one synthesis request.
type SynthesisOptions struct {
	// Voice selects an entry in voices-v1.1-zh.bin. An empty value selects
	// zf_001 for Chinese or mixed text and af_maple for pure English text.
	Voice string
	// Speed is a positive playback-rate multiplier. Zero uses 1.0.
	Speed float32
	// Language accepts auto, zh, or en. The zero value uses auto detection.
	Language Language
	// TrimSilence removes low-amplitude samples from both ends of each segment.
	TrimSilence bool
}

// SynthesisSampleRate is the sample rate of Kokoro float32 PCM output.
const SynthesisSampleRate = 24000
