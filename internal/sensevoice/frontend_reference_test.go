package sensevoice

import (
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/apexracing/tracklogic-asr/internal/audio"
)

func TestFrontendMatchesKaldiReference(t *testing.T) {
	modelDir := filepath.FromSlash("../../models/sensevoice-small-int8")
	cmvn := filepath.Join(modelDir, "am.mvn")
	wav := filepath.FromSlash("../../testdata/zh.wav")
	for _, path := range []string{cmvn, wav} {
		if _, err := os.Stat(path); err != nil {
			t.Skipf("reference asset unavailable: %s", path)
		}
	}
	fe, err := newFrontend(cmvn)
	if err != nil {
		t.Fatal(err)
	}
	samples, sr, err := audio.ReadWAV(wav)
	if err != nil {
		t.Fatal(err)
	}
	features, frames, err := fe.extract(samples, sr)
	if err != nil {
		t.Fatal(err)
	}
	if frames != 93 {
		t.Fatalf("frames=%d", frames)
	}
	want := []float32{-1.3201878, -1.5251820, -1.7011591, -1.6771269, -1.7627099}
	for i := range want {
		if math.Abs(float64(features[i]-want[i])) > 2e-5 {
			t.Fatalf("feature[%d]=%f want %f", i, features[i], want[i])
		}
	}
}
