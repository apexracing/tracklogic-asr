package sensevoice

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"unicode"
)

type decoder struct {
	tokens []string
}

func newDecoder(path string) (*decoder, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read tokens: %w", err)
	}
	var tokens []string
	if err = json.Unmarshal(b, &tokens); err != nil {
		return nil, fmt.Errorf("parse tokens: %w", err)
	}
	if len(tokens) == 0 {
		return nil, fmt.Errorf("token list is empty")
	}
	return &decoder{tokens: tokens}, nil
}

func (d *decoder) decode(logits []float32, frames, vocab int) (Result, error) {
	if frames <= 0 || vocab <= 0 || len(logits) < frames*vocab {
		return Result{}, fmt.Errorf("invalid logits shape [%d,%d] for %d values", frames, vocab, len(logits))
	}
	ids := make([]int, 0, frames)
	prev := -1
	for t := 0; t < frames; t++ {
		row := logits[t*vocab : (t+1)*vocab]
		best := 0
		for i := 1; i < len(row); i++ {
			if row[i] > row[best] {
				best = i
			}
		}
		if best != 0 && best != prev {
			ids = append(ids, best)
		}
		prev = best
	}

	result := Result{TokenIDs: ids}
	var textTokens []string
	for _, id := range ids {
		if id < 0 || id >= len(d.tokens) {
			continue
		}
		tok := d.tokens[id]
		result.Tokens = append(result.Tokens, tok)
		if strings.HasPrefix(tok, "<|") && strings.HasSuffix(tok, "|>") {
			label := strings.TrimSuffix(strings.TrimPrefix(tok, "<|"), "|>")
			switch label {
			case "zh", "en", "yue", "ja", "ko":
				if result.Language == "" {
					result.Language = label
				}
			case "HAPPY", "SAD", "ANGRY", "NEUTRAL", "FEARFUL", "DISGUSTED", "SURPRISED", "OTHER", "EMO_UNKNOWN":
				result.Emotion = label
			case "Speech", "/Speech", "withitn", "woitn":
			default:
				result.Events = append(result.Events, label)
			}
			continue
		}
		if tok != "<unk>" && tok != "<s>" && tok != "</s>" {
			textTokens = append(textTokens, tok)
		}
	}
	result.Text = joinSentencePiece(textTokens)
	return result, nil
}

func joinSentencePiece(tokens []string) string {
	s := strings.TrimSpace(strings.ReplaceAll(strings.Join(tokens, ""), "▁", " "))
	var out []rune
	for _, r := range []rune(s) {
		if unicode.IsPunct(r) && len(out) > 0 && out[len(out)-1] == ' ' {
			out = out[:len(out)-1]
		}
		out = append(out, r)
	}
	return string(out)
}
