package voice

// VoiceGender identifies the presentation category of a selectable voice.
type VoiceGender string

const (
	VoiceGenderFemale VoiceGender = "female"
	VoiceGenderMale   VoiceGender = "male"
)

// VoiceOption describes a curated voice exposed to an application UI.
type VoiceOption struct {
	ID       string      `json:"id"`
	Name     string      `json:"name"`
	Language Language    `json:"language"`
	Locale   string      `json:"locale"`
	Gender   VoiceGender `json:"gender"`
}

var selectableVoices = []VoiceOption{
	{ID: "zf_022", Name: "晴岚", Language: LanguageChinese, Locale: "zh-CN", Gender: VoiceGenderFemale},
	{ID: "zf_001", Name: "若溪", Language: LanguageChinese, Locale: "zh-CN", Gender: VoiceGenderFemale},
	{ID: "zf_046", Name: "凌音", Language: LanguageChinese, Locale: "zh-CN", Gender: VoiceGenderFemale},
	{ID: "zm_058", Name: "长风", Language: LanguageChinese, Locale: "zh-CN", Gender: VoiceGenderMale},
	{ID: "zm_069", Name: "观澜", Language: LanguageChinese, Locale: "zh-CN", Gender: VoiceGenderMale},
	{ID: "zm_009", Name: "知衡", Language: LanguageChinese, Locale: "zh-CN", Gender: VoiceGenderMale},
	{ID: "af_maple", Name: "Maple", Language: LanguageEnglish, Locale: "en-US", Gender: VoiceGenderFemale},
	{ID: "bf_vale", Name: "Vale", Language: LanguageEnglish, Locale: "en-GB", Gender: VoiceGenderFemale},
}

// SelectableVoices returns the curated voices intended for application
// selection. The returned slice is an independent copy and may be modified.
func SelectableVoices() []VoiceOption {
	return append([]VoiceOption(nil), selectableVoices...)
}
