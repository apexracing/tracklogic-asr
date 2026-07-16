package kokoro

import (
	"archive/zip"
	"encoding/binary"
	"math"
	"os"
	"path/filepath"
	"testing"
)

func TestVoiceArchiveReadsNPYAndCachesRows(t *testing.T) {
	path := filepath.Join(t.TempDir(), "voices.bin")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	z := zip.NewWriter(f)
	w, err := z.Create("zf_test.npy")
	if err != nil {
		t.Fatal(err)
	}
	header := "{'descr': '<f4', 'fortran_order': False, 'shape': (510, 1, 256), }"
	padding := 16 - ((10 + len(header) + 1) % 16)
	header += string(make([]byte, padding)) + "\n"
	_, _ = w.Write([]byte("\x93NUMPY\x01\x00"))
	var length [2]byte
	binary.LittleEndian.PutUint16(length[:], uint16(len(header)))
	_, _ = w.Write(length[:])
	_, _ = w.Write([]byte(header))
	var word [4]byte
	for i := 0; i < voiceRows*styleDim; i++ {
		binary.LittleEndian.PutUint32(word[:], math.Float32bits(float32(i)))
		_, _ = w.Write(word[:])
	}
	if err = z.Close(); err != nil {
		t.Fatal(err)
	}
	if err = f.Close(); err != nil {
		t.Fatal(err)
	}

	archive, err := openVoiceArchive(path)
	if err != nil {
		t.Fatal(err)
	}
	defer archive.close()
	if names := archive.names(); len(names) != 1 || names[0] != "zf_test" {
		t.Fatalf("names=%v", names)
	}
	row, err := archive.style("zf_test", 7)
	if err != nil {
		t.Fatal(err)
	}
	if len(row) != styleDim || row[0] != 7*styleDim || row[255] != 7*styleDim+255 {
		t.Fatalf("unexpected row: length=%d first=%v last=%v", len(row), row[0], row[len(row)-1])
	}
	row[0] = -1
	again, err := archive.style("zf_test", 7)
	if err != nil || again[0] == -1 {
		t.Fatalf("cached row was not defensively copied: first=%v err=%v", again[0], err)
	}
}
