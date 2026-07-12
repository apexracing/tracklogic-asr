package sensevoice

import "testing"

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

func TestResampleLinear(t *testing.T) {
	got := resampleLinear([]float32{0, 1, 0}, 3, 6)
	if len(got) != 6 || got[1] != 0.5 || got[2] != 1 {
		t.Fatalf("unexpected resampling: %v", got)
	}
}
