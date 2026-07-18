package g2p

import (
	"strings"
	"testing"
)

func TestChineseToneSandhiAndNumberNormalization(t *testing.T) {
	got, err := phonemizeChinese("你好，100%。")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "ㄋㄧ2ㄏㄠ3") {
		t.Fatalf("third-tone sandhi missing: %q", got)
	}
	if strings.ContainsRune(got, '%') {
		t.Fatalf("number normalization left a percent sign: %q", got)
	}
}

func TestChineseZeroInitialWAndY(t *testing.T) {
	for _, syllable := range []string{"wen2", "wu3", "yuan2", "ying1"} {
		if _, err := pinyinToKokoro(syllable); err != nil {
			t.Fatalf("%s: %v", syllable, err)
		}
	}
}

func TestChineseNumberNormalization(t *testing.T) {
	tests := map[string]string{
		"0":     "零",
		"10":    "十",
		"100":   "一百",
		"1001":  "一千零一",
		"2026":  "二千零二十六",
		"10001": "一万零一",
		"12.5%": "百分之十二点五",
	}
	for input, want := range tests {
		if got := normalizeChineseNumbers(input); got != want {
			t.Fatalf("normalizeChineseNumbers(%q)=%q want %q", input, got, want)
		}
	}
}

func TestEnglishCMUDictAndOOVFallback(t *testing.T) {
	for _, text := range []string{"Hello, world!", "Tracklogic", "GPU 2026"} {
		got, err := phonemizeEnglish(text)
		if err != nil {
			t.Fatalf("%q: %v", text, err)
		}
		if strings.TrimSpace(got) == "" {
			t.Fatalf("%q produced empty phonemes", text)
		}
	}
}

func TestAutoLanguageSegmentation(t *testing.T) {
	f := New()
	tests := []struct {
		text string
		want string
	}{
		{"Hello world.", LanguageEnglish},
		{"GPU 2026.", LanguageEnglish},
		{"千里之行。", LanguageChinese},
		{"你好 Tracklogic!", LanguageAuto},
	}
	for _, test := range tests {
		phones, detected, err := f.Phonemize(test.text, LanguageAuto)
		if err != nil {
			t.Fatalf("%q: %v", test.text, err)
		}
		if phones == "" || detected != test.want {
			t.Fatalf("%q => phones=%q language=%q, want %q", test.text, phones, detected, test.want)
		}
	}
}

func TestAutoLanguageSegmentationPreservesTelemetryDecimals(t *testing.T) {
	chinesePoint, err := phonemizeChinese("点")
	if err != nil {
		t.Fatal(err)
	}
	got, _, err := New().Phonemize("当前油量 62.8 L，速度 213.4 km/h。", LanguageAuto)
	if err != nil {
		t.Fatal(err)
	}
	if count := strings.Count(got, chinesePoint); count != 2 {
		t.Fatalf("Chinese telemetry decimals spoken with %d point markers, want 2: %q", count, got)
	}

	englishPoint, err := phonemizeEnglish("point")
	if err != nil {
		t.Fatal(err)
	}
	got, _, err = New().Phonemize("Fuel 62.8 L, speed 213.4 km/h.", LanguageAuto)
	if err != nil {
		t.Fatal(err)
	}
	if count := strings.Count(got, englishPoint); count != 2 {
		t.Fatalf("English telemetry decimals spoken with %d point markers, want 2: %q", count, got)
	}
}

func TestRejectUnsupportedLanguage(t *testing.T) {
	if _, _, err := New().Phonemize("hello", "fr"); err == nil {
		t.Fatal("expected unsupported-language error")
	}
}
