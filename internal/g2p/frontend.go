package g2p

import (
	"fmt"
	"strings"
	"unicode"
)

const (
	LanguageAuto    = "auto"
	LanguageChinese = "zh"
	LanguageEnglish = "en"
)

// Frontend converts Chinese and US English text to Kokoro phonemes.
type Frontend struct{}

func New() *Frontend { return &Frontend{} }

// Phonemize returns Kokoro phonemes and the detected language.
func (f *Frontend) Phonemize(text, language string) (string, string, error) {
	text = strings.TrimSpace(text)
	if text == "" {
		return "", language, fmt.Errorf("text is empty")
	}
	switch language {
	case "", LanguageAuto:
		return f.phonemizeAuto(text)
	case LanguageChinese:
		p, err := phonemizeChinese(text)
		return p, LanguageChinese, err
	case LanguageEnglish:
		p, err := phonemizeEnglish(text)
		return p, LanguageEnglish, err
	default:
		return "", language, fmt.Errorf("unsupported synthesis language %q", language)
	}
}

type scriptClass byte

const (
	classOther scriptClass = iota
	classChinese
	classEnglish
)

func classify(r rune) scriptClass {
	if unicode.Is(unicode.Han, r) {
		return classChinese
	}
	if unicode.IsLetter(r) && r <= unicode.MaxASCII {
		return classEnglish
	}
	return classOther
}

func (f *Frontend) phonemizeAuto(text string) (string, string, error) {
	var out strings.Builder
	var segment []rune
	runes := []rune(text)
	current := classOther
	lastLexical := classOther
	seenZH, seenEN := false, false
	flush := func() error {
		if len(segment) == 0 {
			return nil
		}
		raw := string(segment)
		var phonemes string
		var err error
		if current == classEnglish {
			phonemes, err = phonemizeEnglish(raw)
			seenEN = true
		} else {
			phonemes, err = phonemizeChinese(raw)
			seenZH = true
		}
		if err != nil {
			return err
		}
		if out.Len() > 0 && phonemes != "" && !strings.HasSuffix(out.String(), " ") {
			out.WriteByte(' ')
		}
		out.WriteString(phonemes)
		segment = segment[:0]
		return nil
	}
	for i, r := range runes {
		class := classify(r)
		if class == classOther {
			if unicode.IsDigit(r) {
				if current == classOther {
					current = lastLexical
					if current == classOther {
						current = classChinese
					}
				}
				segment = append(segment, r)
				continue
			}
			// A decimal separator between digits belongs to the current numeric
			// segment. Treating it as sentence punctuation drops the spoken
			// "point/点" in mixed telemetry text such as "62.8 L".
			if isDecimalSeparator(r) && len(segment) > 0 && unicode.IsDigit(segment[len(segment)-1]) && i+1 < len(runes) && unicode.IsDigit(runes[i+1]) {
				segment = append(segment, '.')
				continue
			}
			if err := flush(); err != nil {
				return "", "", err
			}
			mapped := mapPunctuation(r)
			if mapped != 0 {
				out.WriteRune(mapped)
			}
			current = classOther
			continue
		}
		if current != classOther && current != class {
			if err := flush(); err != nil {
				return "", "", err
			}
		}
		current = class
		lastLexical = class
		segment = append(segment, r)
	}
	if err := flush(); err != nil {
		return "", "", err
	}
	detected := LanguageChinese
	if seenEN && !seenZH {
		detected = LanguageEnglish
	} else if seenEN && seenZH {
		detected = LanguageAuto
	}
	return strings.TrimSpace(out.String()), detected, nil
}

func isDecimalSeparator(r rune) bool {
	return r == '.' || r == '．'
}

func mapPunctuation(r rune) rune {
	switch r {
	case '，', '、':
		return ','
	case '。', '．':
		return '.'
	case '！':
		return '!'
	case '？':
		return '?'
	case '；':
		return ';'
	case '：':
		return ':'
	case '（':
		return '('
	case '）':
		return ')'
	case '“', '”', '"':
		return '"'
	case ',', '.', '!', '?', ';', ':', '(', ')', '—', '…', '/':
		return r
	default:
		if unicode.IsSpace(r) {
			return ' '
		}
		return 0
	}
}
