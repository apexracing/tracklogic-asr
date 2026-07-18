package g2p

import (
	"bufio"
	"bytes"
	"compress/gzip"
	_ "embed"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"unicode"
)

//go:embed cmudict.dict.gz
var cmuCompressed []byte

var (
	cmuOnce        sync.Once
	cmu            map[string][]string
	cmuErr         error
	englishTokenRE = regexp.MustCompile(`[A-Za-z]+(?:['-][A-Za-z]+)*|[0-9]+(?:\.[0-9]+)?|[.,!?;:]`)
)

var arpabet = map[string]string{
	"AA": "ɑ", "AE": "æ", "AH": "ʌ", "AO": "ɔ", "AW": "W", "AY": "Y", "EH": "ɛ", "ER": "ɜɹ",
	"EY": "A", "IH": "ɪ", "IY": "i", "OW": "O", "OY": "Q", "UH": "ʊ", "UW": "u",
	"B": "b", "CH": "ʧ", "D": "d", "DH": "ð", "F": "f", "G": "ɡ", "HH": "h", "JH": "ʤ", "K": "k",
	"L": "l", "M": "m", "N": "n", "NG": "ŋ", "P": "p", "R": "ɹ", "S": "s", "SH": "ʃ", "T": "t",
	"TH": "θ", "V": "v", "W": "w", "Y": "j", "Z": "z", "ZH": "ʒ",
}

var letterARP = map[rune][]string{
	'A': {"EY1"}, 'B': {"B", "IY1"}, 'C': {"S", "IY1"}, 'D': {"D", "IY1"}, 'E': {"IY1"}, 'F': {"EH1", "F"},
	'G': {"JH", "IY1"}, 'H': {"EY1", "CH"}, 'I': {"AY1"}, 'J': {"JH", "EY1"}, 'K': {"K", "EY1"},
	'L': {"EH1", "L"}, 'M': {"EH1", "M"}, 'N': {"EH1", "N"}, 'O': {"OW1"}, 'P': {"P", "IY1"},
	'Q': {"K", "Y", "UW1"}, 'R': {"AA1", "R"}, 'S': {"EH1", "S"}, 'T': {"T", "IY1"}, 'U': {"Y", "UW1"},
	'V': {"V", "IY1"}, 'W': {"D", "AH1", "B", "AH0", "L", "Y", "UW0"}, 'X': {"EH1", "K", "S"},
	'Y': {"W", "AY1"}, 'Z': {"Z", "IY1"},
}

func loadCMU() (map[string][]string, error) {
	cmuOnce.Do(func() {
		reader, err := gzip.NewReader(bytes.NewReader(cmuCompressed))
		if err != nil {
			cmuErr = err
			return
		}
		defer reader.Close()
		cmu = map[string][]string{}
		scanner := bufio.NewScanner(reader)
		for scanner.Scan() {
			fields := strings.Fields(scanner.Text())
			if len(fields) < 2 {
				continue
			}
			word := strings.ToLower(fields[0])
			if strings.Contains(word, "(") {
				continue
			}
			cmu[word] = append([]string(nil), fields[1:]...)
		}
		cmuErr = scanner.Err()
	})
	return cmu, cmuErr
}

func phonemizeEnglish(text string) (string, error) {
	dict, err := loadCMU()
	if err != nil {
		return "", fmt.Errorf("load CMUdict: %w", err)
	}
	tokens := englishTokenRE.FindAllString(text, -1)
	if len(tokens) == 0 {
		return "", fmt.Errorf("text contains no English words")
	}
	var out []string
	for _, token := range tokens {
		if len(token) == 1 && strings.ContainsRune(".,!?;:", rune(token[0])) {
			out = append(out, token)
			continue
		}
		if isEnglishNumber(token) {
			token = englishNumber(token)
		}
		words := strings.Fields(strings.ReplaceAll(token, "-", " "))
		for _, word := range words {
			var phones []string
			if isAcronym(word) {
				for _, letter := range word {
					phones = append(phones, letterARP[unicode.ToUpper(letter)]...)
				}
			} else if value := dict[strings.ToLower(word)]; value != nil {
				phones = value
			} else {
				phones = nrlFallback(word)
			}
			ipa, err := arpabetToKokoro(phones)
			if err != nil {
				return "", fmt.Errorf("phonemize %q: %w", word, err)
			}
			out = append(out, ipa)
		}
	}
	return strings.Join(out, " "), nil
}

func isAcronym(word string) bool {
	letters := 0
	for _, r := range word {
		if unicode.IsLetter(r) {
			letters++
			if !unicode.IsUpper(r) {
				return false
			}
		}
	}
	return letters > 1
}

func arpabetToKokoro(phones []string) (string, error) {
	var out strings.Builder
	for _, phone := range phones {
		stress := byte(0)
		if len(phone) > 0 {
			last := phone[len(phone)-1]
			if last >= '0' && last <= '2' {
				stress = last
				phone = phone[:len(phone)-1]
			}
		}
		mapped, ok := arpabet[phone]
		if !ok {
			return "", fmt.Errorf("unsupported ARPAbet %q", phone)
		}
		if phone == "AH" && stress == '0' {
			mapped = "ə"
		}
		if phone == "ER" && stress == '0' {
			mapped = "ɚ"
		}
		if stress == '1' {
			out.WriteRune('ˈ')
		} else if stress == '2' {
			out.WriteRune('ˌ')
		}
		out.WriteString(mapped)
	}
	return out.String(), nil
}

// nrlFallback is a compact NRL-inspired fallback for words absent from CMUdict.
func nrlFallback(word string) []string {
	w := strings.ToLower(strings.Trim(word, "'"))
	patterns := []struct {
		text   string
		phones []string
	}{
		{"tion", []string{"SH", "AH0", "N"}}, {"ough", []string{"OW1"}}, {"igh", []string{"AY1"}},
		{"ch", []string{"CH"}}, {"sh", []string{"SH"}}, {"th", []string{"TH"}}, {"ph", []string{"F"}},
		{"ng", []string{"NG"}}, {"qu", []string{"K", "W"}}, {"ck", []string{"K"}}, {"ee", []string{"IY1"}},
		{"oo", []string{"UW1"}}, {"ai", []string{"EY1"}}, {"ay", []string{"EY1"}}, {"ow", []string{"OW1"}},
	}
	var out []string
	stressed := false
	for i := 0; i < len(w); {
		matched := false
		for _, pattern := range patterns {
			if strings.HasPrefix(w[i:], pattern.text) {
				out = append(out, pattern.phones...)
				i += len(pattern.text)
				matched = true
				stressed = stressed || containsStress(pattern.phones)
				break
			}
		}
		if matched {
			continue
		}
		c := w[i]
		i++
		phone := map[byte]string{'a': "AE", 'b': "B", 'c': "K", 'd': "D", 'e': "EH", 'f': "F", 'g': "G", 'h': "HH", 'i': "IH", 'j': "JH", 'k': "K", 'l': "L", 'm': "M", 'n': "N", 'o': "AA", 'p': "P", 'q': "K", 'r': "R", 's': "S", 't': "T", 'u': "AH", 'v': "V", 'w': "W", 'x': "K S", 'y': "Y", 'z': "Z"}[c]
		for _, p := range strings.Fields(phone) {
			if !stressed && strings.Contains("AE EH IH AA AH", p) {
				p += "1"
				stressed = true
			}
			out = append(out, p)
		}
	}
	if len(out) == 0 {
		return []string{"AH0"}
	}
	return out
}

func containsStress(phones []string) bool {
	for _, p := range phones {
		if strings.ContainsAny(p, "12") {
			return true
		}
	}
	return false
}

func englishNumber(raw string) string {
	parts := strings.SplitN(raw, ".", 2)
	if len(parts) == 2 {
		integer := englishInteger(parts[0])
		fraction := make([]string, 0, len(parts[1]))
		for _, r := range parts[1] {
			fraction = append(fraction, digitWord(r))
		}
		return integer + " point " + strings.Join(fraction, " ")
	}
	return englishInteger(raw)
}

func isEnglishNumber(raw string) bool {
	parts := strings.SplitN(raw, ".", 2)
	if len(parts) == 0 || parts[0] == "" {
		return false
	}
	if _, err := strconv.Atoi(parts[0]); err != nil {
		return false
	}
	if len(parts) == 1 {
		return true
	}
	if parts[1] == "" {
		return false
	}
	_, err := strconv.Atoi(parts[1])
	return err == nil
}

func englishInteger(raw string) string {
	n, err := strconv.Atoi(raw)
	if err != nil {
		return raw
	}
	if n == 0 {
		return "zero"
	}
	if n < 0 || n > 999999 {
		var parts []string
		for _, r := range raw {
			parts = append(parts, digitWord(r))
		}
		return strings.Join(parts, " ")
	}
	var parts []string
	if n >= 1000 {
		parts = append(parts, smallEnglishNumber(n/1000), "thousand")
		n %= 1000
	}
	if n > 0 {
		parts = append(parts, smallEnglishNumber(n))
	}
	return strings.Join(parts, " ")
}

func smallEnglishNumber(n int) string {
	ones := []string{"", "one", "two", "three", "four", "five", "six", "seven", "eight", "nine", "ten", "eleven", "twelve", "thirteen", "fourteen", "fifteen", "sixteen", "seventeen", "eighteen", "nineteen"}
	tens := []string{"", "", "twenty", "thirty", "forty", "fifty", "sixty", "seventy", "eighty", "ninety"}
	var parts []string
	if n >= 100 {
		parts = append(parts, ones[n/100], "hundred")
		n %= 100
	}
	if n >= 20 {
		parts = append(parts, tens[n/10])
		n %= 10
	}
	if n > 0 {
		parts = append(parts, ones[n])
	}
	return strings.Join(parts, " ")
}

func digitWord(r rune) string {
	values := []string{"zero", "one", "two", "three", "four", "five", "six", "seven", "eight", "nine"}
	if r >= '0' && r <= '9' {
		return values[r-'0']
	}
	return ""
}
