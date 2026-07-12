package sensevoice

import "testing"

func TestFrontendRejectsShortAudio(t *testing.T) {
	if _, _, err := (&frontend{}).extract(make([]float32, 100), 16000); err == nil {
		t.Fatal("expected short-audio error")
	}
}

func TestDecoderCTCGreedyAndMetadata(t *testing.T) {
	d := &decoder{tokens: []string{"<blank>", "<|zh|>", "<|NEUTRAL|>", "▁hello", "世", "界"}}
	ids := []int{1, 1, 0, 2, 3, 3, 0, 4, 5}
	logits := make([]float32, len(ids)*len(d.tokens))
	for frame, id := range ids {
		logits[frame*len(d.tokens)+id] = 10
	}
	got, err := d.decode(logits, len(ids), len(d.tokens))
	if err != nil {
		t.Fatal(err)
	}
	if got.Text != "hello世界" {
		t.Fatalf("text = %q", got.Text)
	}
	if got.Language != "zh" || got.Emotion != "NEUTRAL" {
		t.Fatalf("metadata = language %q emotion %q", got.Language, got.Emotion)
	}
}
