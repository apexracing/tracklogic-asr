package g2p

import (
	"fmt"
	"regexp"
	"strings"
	"sync"
	"unicode"

	"github.com/go-ego/gse"
	pinyin "github.com/mozillazg/go-pinyin"
)

var (
	segmenterOnce  sync.Once
	segmenter      gse.Segmenter
	segmenterErr   error
	percentRE      = regexp.MustCompile(`([0-9]+(?:\.[0-9]+)?)%`)
	numberRE       = regexp.MustCompile(`[0-9]+(?:\.[0-9]+)?`)
	zhUnitPatterns = []zhUnitPattern{
		newZHUnitPattern(`kg/(?:m\^?3|m³)`, "千克每立方米"),
		newZHUnitPattern(`m/(?:s\^?2|s²)`, "米每二次方秒"),
		newZHUnitPattern(`(?:km/h|kmh|kph)`, "公里每小时"),
		newZHUnitPattern(`mph`, "英里每小时"),
		newZHUnitPattern(`kg/h`, "千克每小时"),
		newZHUnitPattern(`rad/s`, "弧度每秒"),
		newZHUnitPattern(`(?:revs/min|rpm)`, "转每分钟"),
		newZHUnitPattern(`m/s`, "米每秒"),
		newZHUnitPattern(`s/s`, "秒每秒"),
		newZHUnitPattern(`(?:N\s*[*·]\s*m|Nm)`, "牛顿米"),
		newZHUnitPattern(`kWh`, "千瓦时"),
		newZHUnitPattern(`kPa`, "千帕"),
		newZHUnitPattern(`mmHg`, "毫米汞柱"),
		newZHUnitPattern(`inHg`, "英寸汞柱"),
		newZHUnitPattern(`psi`, "磅每平方英寸"),
		newZHUnitPattern(`bar`, "巴"),
		newZHUnitPattern(`(?:°\s*C|℃)`, "摄氏度"),
		newZHUnitPattern(`(?:°\s*F|℉)`, "华氏度"),
		newZHUnitPattern(`fps`, "帧每秒"),
		newZHUnitPattern(`Hz`, "赫兹"),
		newZHUnitPattern(`kW`, "千瓦"),
		newZHUnitPattern(`hp`, "马力"),
		newZHUnitPattern(`km`, "公里"),
		newZHUnitPattern(`cm`, "厘米"),
		newZHUnitPattern(`mm`, "毫米"),
		newZHUnitPattern(`ms`, "毫秒"),
		newZHUnitPattern(`kg`, "千克"),
		newZHUnitPattern(`kJ`, "千焦"),
		newZHUnitPattern(`Pa`, "帕"),
		newZHUnitPattern(`min`, "分钟"),
		newZHUnitPattern(`(?:L|l)`, "升"),
		newZHUnitPattern(`rad`, "弧度"),
		newZHUnitPattern(`V`, "伏"),
		newZHUnitPattern(`C`, "摄氏度"),
		newZHUnitPattern(`F`, "华氏度"),
		newZHUnitPattern(`m`, "米"),
		newZHUnitPattern(`s`, "秒"),
		newZHUnitPattern(`h`, "小时"),
	}
)

type zhUnitPattern struct {
	re          *regexp.Regexp
	replacement string
}

func newZHUnitPattern(unitPattern, spoken string) zhUnitPattern {
	return zhUnitPattern{
		re:          regexp.MustCompile(`(?i)([0-9]+(?:\.[0-9]+)?)\s*(?:` + unitPattern + `)($|[^A-Za-z0-9])`),
		replacement: `${1}` + spoken + `${2}`,
	}
}

var zhInitial = map[string]string{
	"b": "ㄅ", "p": "ㄆ", "m": "ㄇ", "f": "ㄈ", "d": "ㄉ", "t": "ㄊ", "n": "ㄋ", "l": "ㄌ",
	"g": "ㄍ", "k": "ㄎ", "h": "ㄏ", "j": "ㄐ", "q": "ㄑ", "x": "ㄒ", "zh": "ㄓ", "ch": "ㄔ",
	"sh": "ㄕ", "r": "ㄖ", "z": "ㄗ", "c": "ㄘ", "s": "ㄙ",
}

var zhFinal = map[string]string{
	"a": "ㄚ", "o": "ㄛ", "e": "ㄜ", "ie": "ㄝ", "ai": "ㄞ", "ei": "ㄟ", "ao": "ㄠ", "ou": "ㄡ",
	"an": "ㄢ", "en": "ㄣ", "ang": "ㄤ", "eng": "ㄥ", "er": "ㄦ", "i": "ㄧ", "u": "ㄨ", "v": "ㄩ",
	"ii": "ㄭ", "iii": "十", "ve": "月", "ia": "压", "ian": "言", "iang": "阳", "iao": "要",
	"in": "阴", "ing": "应", "iong": "用", "iou": "又", "ong": "中", "ua": "穵", "uai": "外",
	"uan": "万", "uang": "王", "uei": "为", "uen": "文", "ueng": "瓮", "uo": "我", "van": "元", "vn": "云",
}

var phrasePinyin = map[string][]string{
	"银行": {"yin2", "hang2"}, "行走": {"xing2", "zou3"}, "重庆": {"chong2", "qing4"},
	"音乐": {"yin1", "yue4"}, "长大": {"zhang3", "da4"}, "还是": {"hai2", "shi4"},
}

func chineseSegmenter() (*gse.Segmenter, error) {
	segmenterOnce.Do(func() { segmenter, segmenterErr = gse.NewEmbed("zh") })
	return &segmenter, segmenterErr
}

type zhPart struct {
	word      string
	syllables []string
	punct     rune
}

func phonemizeChinese(text string) (string, error) {
	text = normalizeChineseNumbers(text)
	seg, err := chineseSegmenter()
	if err != nil {
		return "", fmt.Errorf("initialize Chinese segmenter: %w", err)
	}
	words := seg.Cut(text, true)
	parts := make([]zhPart, 0, len(words))
	args := pinyin.NewArgs()
	args.Style = pinyin.Tone3
	args.Fallback = func(r rune, _ pinyin.Args) []string { return []string{string(r)} }
	for _, word := range words {
		if strings.TrimSpace(word) == "" {
			continue
		}
		runes := []rune(word)
		if len(runes) == 1 && !unicode.Is(unicode.Han, runes[0]) {
			if p := mapPunctuation(runes[0]); p != 0 {
				parts = append(parts, zhPart{punct: p})
			}
			continue
		}
		syllables := phrasePinyin[word]
		if syllables == nil {
			result := pinyin.Pinyin(word, args)
			for _, values := range result {
				if len(values) > 0 {
					syllables = append(syllables, ensureTone(values[0]))
				}
			}
		}
		if len(syllables) == 0 {
			return "", fmt.Errorf("cannot phonemize Chinese word %q", word)
		}
		applyNeutralTone(runes, syllables)
		applyWordSandhi(runes, syllables)
		parts = append(parts, zhPart{word: word, syllables: syllables})
	}
	applyThirdToneSandhi(parts)
	var out strings.Builder
	for i, part := range parts {
		if part.punct != 0 {
			out.WriteRune(part.punct)
			continue
		}
		for j, syllable := range part.syllables {
			phones, err := pinyinToKokoro(syllable)
			if err != nil {
				return "", fmt.Errorf("phonemize %q: %w", part.word, err)
			}
			if j == len(part.syllables)-1 && strings.HasSuffix(part.word, "儿") && len(part.syllables) > 1 {
				phones = strings.TrimSuffix(phones, phones[len(phones)-1:]) + "R" + phones[len(phones)-1:]
			}
			out.WriteString(phones)
		}
		if i+1 < len(parts) && parts[i+1].punct == 0 {
			out.WriteRune('/')
		}
	}
	return out.String(), nil
}

func normalizeChineseNumbers(text string) string {
	text = normalizeChineseMeasurements(text)
	return numberRE.ReplaceAllStringFunc(text, chineseNumber)
}

// normalizeChineseMeasurements converts telemetry-oriented symbols and SI
// abbreviations while their numeric context is still available. Matching a
// leading number and a complete unit token avoids rewriting car names and
// ordinary acronyms such as GT3 or LMP2.
func normalizeChineseMeasurements(text string) string {
	text = strings.NewReplacer("．", ".", "％", "%").Replace(text)
	text = percentRE.ReplaceAllString(text, "百分之$1")
	for _, pattern := range zhUnitPatterns {
		text = pattern.re.ReplaceAllString(text, pattern.replacement)
	}
	return text
}

func chineseNumber(raw string) string {
	parts := strings.SplitN(raw, ".", 2)
	integer := strings.TrimLeft(parts[0], "0")
	var out string
	if integer == "" {
		out = "零"
	} else if len(integer) <= 12 {
		var value int64
		for _, r := range integer {
			value = value*10 + int64(r-'0')
		}
		out = chineseInteger(value)
	} else {
		out = chineseDigits(parts[0])
	}
	if len(parts) == 2 {
		out += "点" + chineseDigits(parts[1])
	}
	return out
}

func chineseInteger(value int64) string {
	if value == 0 {
		return "零"
	}
	if value >= 100000000 {
		high, rest := value/100000000, value%100000000
		out := chineseInteger(high) + "亿"
		if rest > 0 {
			if rest < 10000000 {
				out += "零"
			}
			out += chineseInteger(rest)
		}
		return out
	}
	if value >= 10000 {
		high, rest := value/10000, value%10000
		out := chineseBelow10000(high) + "万"
		if rest > 0 {
			if rest < 1000 {
				out += "零"
			}
			out += chineseBelow10000(rest)
		}
		return out
	}
	return chineseBelow10000(value)
}

func chineseBelow10000(value int64) string {
	digits := []string{"零", "一", "二", "三", "四", "五", "六", "七", "八", "九"}
	units := []string{"千", "百", "十", ""}
	divisors := []int64{1000, 100, 10, 1}
	var out strings.Builder
	pendingZero := false
	for i, divisor := range divisors {
		digit := value / divisor
		value %= divisor
		if digit == 0 {
			if out.Len() > 0 && value > 0 {
				pendingZero = true
			}
			continue
		}
		if pendingZero {
			out.WriteString("零")
			pendingZero = false
		}
		if !(divisor == 10 && digit == 1 && out.Len() == 0) {
			out.WriteString(digits[digit])
		}
		out.WriteString(units[i])
	}
	return out.String()
}

func chineseDigits(raw string) string {
	digits := []string{"零", "一", "二", "三", "四", "五", "六", "七", "八", "九"}
	var out strings.Builder
	for _, r := range raw {
		out.WriteString(digits[r-'0'])
	}
	return out.String()
}

func ensureTone(s string) string {
	if s == "" {
		return s
	}
	last := s[len(s)-1]
	if last < '1' || last > '5' {
		return s + "5"
	}
	return s
}

func tone(s string) byte { return ensureTone(s)[len(ensureTone(s))-1] }

func setTone(s string, value byte) string {
	return ensureTone(s)[:len(ensureTone(s))-1] + string(value)
}

func applyNeutralTone(runes []rune, syllables []string) {
	neutral := "的了着过吗呢吧啊呀嘛么"
	for i, r := range runes {
		if i < len(syllables) && strings.ContainsRune(neutral, r) {
			syllables[i] = setTone(syllables[i], '5')
		}
	}
}

func applyWordSandhi(runes []rune, syllables []string) {
	for i, r := range runes {
		if i >= len(syllables) {
			break
		}
		if r == '不' && i+1 < len(syllables) && tone(syllables[i+1]) == '4' {
			syllables[i] = setTone(syllables[i], '2')
		}
		if r == '一' && i+1 < len(syllables) {
			next := tone(syllables[i+1])
			if next == '4' {
				syllables[i] = setTone(syllables[i], '2')
			} else if next >= '1' && next <= '3' {
				syllables[i] = setTone(syllables[i], '4')
			}
		}
	}
}

func applyThirdToneSandhi(parts []zhPart) {
	type ref struct{ p, s int }
	var run []ref
	flush := func() {
		for _, r := range run[:max(0, len(run)-1)] {
			parts[r.p].syllables[r.s] = setTone(parts[r.p].syllables[r.s], '2')
		}
		run = run[:0]
	}
	for pi := range parts {
		if parts[pi].punct != 0 {
			flush()
			continue
		}
		for si, syllable := range parts[pi].syllables {
			if tone(syllable) == '3' {
				run = append(run, ref{pi, si})
			} else {
				flush()
			}
		}
	}
	flush()
}

func pinyinToKokoro(syllable string) (string, error) {
	syllable = ensureTone(strings.ToLower(syllable))
	toneDigit := syllable[len(syllable)-1:]
	base := syllable[:len(syllable)-1]
	initial := ""
	for _, candidate := range []string{"zh", "ch", "sh", "b", "p", "m", "f", "d", "t", "n", "l", "g", "k", "h", "j", "q", "x", "r", "z", "c", "s"} {
		if strings.HasPrefix(base, candidate) {
			initial = candidate
			base = strings.TrimPrefix(base, candidate)
			break
		}
	}
	if initial == "" {
		base = normalizeZeroInitial(base)
	}
	if initial != "" && strings.Contains("jqx", initial) {
		if base == "u" {
			base = "v"
		} else if strings.HasPrefix(base, "u") {
			base = "v" + base[1:]
		}
	}
	if base == "i" && strings.Contains("zcs", initial) {
		base = "ii"
	} else if base == "i" && (initial == "zh" || initial == "ch" || initial == "sh" || initial == "r") {
		base = "iii"
	}
	switch base {
	case "iu":
		base = "iou"
	case "ui":
		base = "uei"
	case "un":
		base = "uen"
	case "ue":
		base = "ve"
	}
	final, ok := zhFinal[base]
	if !ok {
		return "", fmt.Errorf("unsupported pinyin final %q in %q", base, syllable)
	}
	return zhInitial[initial] + final + toneDigit, nil
}

func normalizeZeroInitial(base string) string {
	switch base {
	case "yi":
		return "i"
	case "ya":
		return "ia"
	case "ye":
		return "ie"
	case "yao":
		return "iao"
	case "you":
		return "iou"
	case "yan":
		return "ian"
	case "yin":
		return "in"
	case "yang":
		return "iang"
	case "ying":
		return "ing"
	case "yong":
		return "iong"
	case "yu":
		return "v"
	case "yue":
		return "ve"
	case "yuan":
		return "van"
	case "yun":
		return "vn"
	case "wu":
		return "u"
	case "wa":
		return "ua"
	case "wo":
		return "uo"
	case "wai":
		return "uai"
	case "wei":
		return "uei"
	case "wan":
		return "uan"
	case "wen":
		return "uen"
	case "wang":
		return "uang"
	case "weng":
		return "ueng"
	default:
		return base
	}
}
