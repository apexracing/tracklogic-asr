package asr

import "github.com/apexracing/tracklogic-asr/assets"

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

// Config controls recognizer resources and ONNX CPU execution.
type Config struct {
	Assets     assets.Config
	NumThreads int
}

// Options controls one transcription request.
type Options struct {
	Language   Language
	WithoutITN bool
}

// Result contains recognized text and SenseVoice metadata.
type Result struct {
	Text     string
	Language string
	Emotion  string
	Events   []string
	Tokens   []string
	TokenIDs []int
}
