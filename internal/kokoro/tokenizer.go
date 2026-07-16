package kokoro

import (
	"encoding/json"
	"fmt"
	"os"
	"unicode/utf8"
)

type tokenizer struct{ vocab map[rune]int64 }

func loadTokenizer(path string) (*tokenizer, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read Kokoro tokenizer: %w", err)
	}
	var document struct {
		Model struct {
			Vocab map[string]int64 `json:"vocab"`
		} `json:"model"`
	}
	if err = json.Unmarshal(b, &document); err != nil {
		return nil, fmt.Errorf("parse Kokoro tokenizer: %w", err)
	}
	t := &tokenizer{vocab: make(map[rune]int64, len(document.Model.Vocab))}
	for text, id := range document.Model.Vocab {
		runes := []rune(text)
		if len(runes) == 1 {
			t.vocab[runes[0]] = id
		}
	}
	if len(t.vocab) == 0 {
		return nil, fmt.Errorf("Kokoro tokenizer has no single-rune vocabulary")
	}
	return t, nil
}

func (t *tokenizer) tokenize(text string) ([]int64, error) {
	ids := make([]int64, 0, utf8.RuneCountInString(text))
	for _, r := range text {
		id, ok := t.vocab[r]
		if !ok {
			return nil, fmt.Errorf("unsupported Kokoro phoneme %q", r)
		}
		ids = append(ids, id)
	}
	return ids, nil
}

func chunkTokens(ids []int64, max int) [][]int64 {
	if len(ids) == 0 {
		return nil
	}
	chunks := make([][]int64, 0, (len(ids)+max-1)/max)
	for len(ids) > max {
		cut := max
		for i := max - 1; i >= max/2; i-- {
			if ids[i] >= 1 && ids[i] <= 6 { // punctuation IDs ;:,.!?
				cut = i + 1
				break
			}
		}
		chunks = append(chunks, ids[:cut])
		ids = ids[cut:]
	}
	if len(ids) > 0 {
		chunks = append(chunks, ids)
	}
	return chunks
}
