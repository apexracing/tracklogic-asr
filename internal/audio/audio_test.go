package audio

import (
	"bytes"
	"encoding/binary"
	"math"
	"os"
	"path/filepath"
	"testing"
)

func TestResampleLinear(t *testing.T) {
	got := ResampleLinear([]float32{0, 1, 0}, 3, 6)
	if len(got) != 6 || got[1] != 0.5 || got[2] != 1 {
		t.Fatalf("unexpected resampling: %v", got)
	}
}

func TestReadWAVPCMDepths(t *testing.T) {
	tests := []struct {
		name         string
		format, bits uint16
		data         []byte
		want         float32
	}{
		{"u8", 1, 8, []byte{192}, 0.5},
		{"s16", 1, 16, []byte{0x00, 0x40}, 0.5},
		{"s24", 1, 24, []byte{0x00, 0x00, 0x40}, 0.5},
		{"s32", 1, 32, []byte{0x00, 0x00, 0x00, 0x40}, 0.5},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := writeTestWAV(t, tt.format, 1, tt.bits, tt.data)
			got, sr, err := ReadWAV(path)
			if err != nil {
				t.Fatal(err)
			}
			if sr != 16000 || len(got) != 1 || math.Abs(float64(got[0]-tt.want)) > 1e-5 {
				t.Fatalf("samples=%v rate=%d", got, sr)
			}
		})
	}
}

func TestReadWAVDownmixesStereoFloat(t *testing.T) {
	var data bytes.Buffer
	_ = binary.Write(&data, binary.LittleEndian, math.Float32bits(1))
	_ = binary.Write(&data, binary.LittleEndian, math.Float32bits(-1))
	path := writeTestWAV(t, 3, 2, 32, data.Bytes())
	got, _, err := ReadWAV(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || math.Abs(float64(got[0])) > 1e-6 {
		t.Fatalf("downmix=%v", got)
	}
}

func writeTestWAV(t *testing.T, format, channels, bits uint16, data []byte) string {
	t.Helper()
	var b bytes.Buffer
	b.WriteString("RIFF")
	_ = binary.Write(&b, binary.LittleEndian, uint32(36+len(data)))
	b.WriteString("WAVEfmt ")
	_ = binary.Write(&b, binary.LittleEndian, uint32(16))
	_ = binary.Write(&b, binary.LittleEndian, format)
	_ = binary.Write(&b, binary.LittleEndian, channels)
	_ = binary.Write(&b, binary.LittleEndian, uint32(16000))
	block := channels * (bits / 8)
	_ = binary.Write(&b, binary.LittleEndian, uint32(16000)*uint32(block))
	_ = binary.Write(&b, binary.LittleEndian, block)
	_ = binary.Write(&b, binary.LittleEndian, bits)
	b.WriteString("data")
	_ = binary.Write(&b, binary.LittleEndian, uint32(len(data)))
	b.Write(data)
	path := filepath.Join(t.TempDir(), "test.wav")
	if err := os.WriteFile(path, b.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}
