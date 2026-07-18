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

func TestChineseTelemetryMeasurementNormalization(t *testing.T) {
	tests := map[string]string{
		"油量 62.8 L":                 "油量 六十二点八升",
		"速度 213.4 km/h":             "速度 二百一十三点四公里每小时",
		"胎压 180.5 kPa，制动压力 3.2 bar": "胎压 一百八十点五千帕，制动压力 三点二巴",
		"赛道温度 42.3 °C，风速 3.2 m/s":   "赛道温度 四十二点三摄氏度，风速 三点二米每秒",
		"横向加速度 9.8 m/s^2":           "横向加速度 九点八米每二次方秒",
		"空气密度 1.225 kg/m^3":         "空气密度 一点二二五千克每立方米",
		"转速 6500 RPM，采样率 360 Hz":    "转速 六千五百转每分钟，采样率 三百六十赫兹",
		"扭矩 500 N*m，电量 30.5 kWh":    "扭矩 五百牛顿米，电量 三十点五千瓦时",
		"距离 3.6 km，用时 83.42 s":      "距离 三点六公里，用时 八十三点四二秒",
		"湿度 54％":                    "湿度 百分之五十四",
	}
	for input, want := range tests {
		if got := normalizeChineseNumbers(input); got != want {
			t.Fatalf("normalizeChineseNumbers(%q)=%q want %q", input, got, want)
		}
	}
}

func TestChineseTelemetryMeasurementNormalizationDoesNotRewriteNames(t *testing.T) {
	input := "2026 S3，Porsche 963 LMDh，BMW M2，GT3"
	if got := normalizeChineseMeasurements(input); got != input {
		t.Fatalf("normalizeChineseMeasurements(%q)=%q", input, got)
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

func TestAutoLanguageSegmentationSpeaksChineseTelemetryUnits(t *testing.T) {
	frontend := New()
	got, _, err := frontend.Phonemize("油量 62.8 L，速度 213.4 km/h，胎压 180.5 kPa，赛道温度 42.3 °C，风速 3.2 m/s。", LanguageAuto)
	if err != nil {
		t.Fatal(err)
	}
	want, _, err := frontend.Phonemize("油量六十二点八升，速度二百一十三点四公里每小时，胎压一百八十点五千帕，赛道温度四十二点三摄氏度，风速三点二米每秒。", LanguageChinese)
	if err != nil {
		t.Fatal(err)
	}
	compact := func(value string) string {
		return strings.NewReplacer(" ", "", "/", "").Replace(value)
	}
	if compact(got) != compact(want) {
		t.Fatalf("mixed telemetry units did not match expanded Chinese speech:\n got %q\nwant %q", got, want)
	}
}

func TestRejectUnsupportedLanguage(t *testing.T) {
	if _, _, err := New().Phonemize("hello", "fr"); err == nil {
		t.Fatal("expected unsupported-language error")
	}
}
