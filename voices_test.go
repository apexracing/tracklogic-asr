package voice

import "testing"

func TestSelectableVoices(t *testing.T) {
	want := []VoiceOption{
		{ID: "zf_022", Name: "晴岚", Language: LanguageChinese, Locale: "zh-CN", Gender: VoiceGenderFemale},
		{ID: "zf_001", Name: "若溪", Language: LanguageChinese, Locale: "zh-CN", Gender: VoiceGenderFemale},
		{ID: "zf_046", Name: "凌音", Language: LanguageChinese, Locale: "zh-CN", Gender: VoiceGenderFemale},
		{ID: "zm_058", Name: "长风", Language: LanguageChinese, Locale: "zh-CN", Gender: VoiceGenderMale},
		{ID: "zm_069", Name: "观澜", Language: LanguageChinese, Locale: "zh-CN", Gender: VoiceGenderMale},
		{ID: "zm_009", Name: "知衡", Language: LanguageChinese, Locale: "zh-CN", Gender: VoiceGenderMale},
		{ID: "af_maple", Name: "Maple", Language: LanguageEnglish, Locale: "en-US", Gender: VoiceGenderFemale},
		{ID: "bf_vale", Name: "Vale", Language: LanguageEnglish, Locale: "en-GB", Gender: VoiceGenderFemale},
	}
	got := SelectableVoices()
	if len(got) != len(want) {
		t.Fatalf("SelectableVoices returned %d entries, want %d", len(got), len(want))
	}
	ids := make(map[string]struct{}, len(got))
	names := make(map[string]struct{}, len(got))
	for i, option := range got {
		if option != want[i] {
			t.Fatalf("SelectableVoices()[%d]=%+v want %+v", i, option, want[i])
		}
		if _, exists := ids[option.ID]; exists {
			t.Fatalf("duplicate selectable voice ID %q", option.ID)
		}
		ids[option.ID] = struct{}{}
		if _, exists := names[option.Name]; exists {
			t.Fatalf("duplicate selectable voice name %q", option.Name)
		}
		names[option.Name] = struct{}{}
	}

	got[0].Name = "changed"
	if SelectableVoices()[0].Name == "changed" {
		t.Fatal("SelectableVoices returned mutable package state")
	}
}
