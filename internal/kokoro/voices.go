package kokoro

import (
	"archive/zip"
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
)

const (
	voiceRows = 510
	styleDim  = 256
)

var shapePattern = regexp.MustCompile(`'shape':\s*\(\s*(\d+)\s*,\s*(?:(\d+)\s*,\s*)?(\d+)\s*,?\s*\)`)

type voiceArchive struct {
	zip   *zip.ReadCloser
	files map[string]*zip.File
	mu    sync.Mutex
	cache map[string][]float32
}

func openVoiceArchive(path string) (*voiceArchive, error) {
	z, err := zip.OpenReader(path)
	if err != nil {
		return nil, fmt.Errorf("open Kokoro voices: %w", err)
	}
	v := &voiceArchive{zip: z, files: map[string]*zip.File{}, cache: map[string][]float32{}}
	for _, file := range z.File {
		if !strings.HasSuffix(file.Name, ".npy") || strings.Contains(file.Name, "/") {
			continue
		}
		name := strings.TrimSuffix(file.Name, ".npy")
		v.files[name] = file
	}
	if len(v.files) == 0 {
		z.Close()
		return nil, fmt.Errorf("Kokoro voices archive contains no .npy entries")
	}
	return v, nil
}

func (v *voiceArchive) names() []string {
	out := make([]string, 0, len(v.files))
	for name := range v.files {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

func (v *voiceArchive) style(name string, tokenCount int) ([]float32, error) {
	if tokenCount < 0 || tokenCount >= voiceRows {
		return nil, fmt.Errorf("token count %d exceeds voice style rows", tokenCount)
	}
	v.mu.Lock()
	defer v.mu.Unlock()
	data, ok := v.cache[name]
	if !ok {
		file := v.files[name]
		if file == nil {
			return nil, fmt.Errorf("unknown voice %q", name)
		}
		var err error
		data, err = readNPY(file)
		if err != nil {
			return nil, fmt.Errorf("read voice %s: %w", name, err)
		}
		v.cache[name] = data
	}
	off := tokenCount * styleDim
	return append([]float32(nil), data[off:off+styleDim]...), nil
}

func (v *voiceArchive) close() error { return v.zip.Close() }

func readNPY(file *zip.File) ([]float32, error) {
	r, err := file.Open()
	if err != nil {
		return nil, err
	}
	defer r.Close()
	var preamble [8]byte
	if _, err = io.ReadFull(r, preamble[:]); err != nil {
		return nil, err
	}
	if string(preamble[:6]) != "\x93NUMPY" {
		return nil, fmt.Errorf("invalid NPY magic")
	}
	major := preamble[6]
	var headerLen uint32
	if major == 1 {
		var b [2]byte
		if _, err = io.ReadFull(r, b[:]); err != nil {
			return nil, err
		}
		headerLen = uint32(binary.LittleEndian.Uint16(b[:]))
	} else if major == 2 || major == 3 {
		var b [4]byte
		if _, err = io.ReadFull(r, b[:]); err != nil {
			return nil, err
		}
		headerLen = binary.LittleEndian.Uint32(b[:])
	} else {
		return nil, fmt.Errorf("unsupported NPY version %d", major)
	}
	if headerLen == 0 || headerLen > 4096 {
		return nil, fmt.Errorf("invalid NPY header length %d", headerLen)
	}
	header := make([]byte, headerLen)
	if _, err = io.ReadFull(r, header); err != nil {
		return nil, err
	}
	hs := string(header)
	if !strings.Contains(hs, "'descr': '<f4'") || !strings.Contains(hs, "'fortran_order': False") {
		return nil, fmt.Errorf("voice NPY must be little-endian float32 in C order")
	}
	match := shapePattern.FindStringSubmatch(hs)
	if match == nil {
		return nil, fmt.Errorf("cannot parse NPY shape")
	}
	dims := make([]int, 0, 3)
	for _, raw := range match[1:] {
		if raw == "" {
			continue
		}
		n, err := strconv.Atoi(raw)
		if err != nil {
			return nil, err
		}
		dims = append(dims, n)
	}
	if !((len(dims) == 3 && dims[0] == voiceRows && dims[1] == 1 && dims[2] == styleDim) ||
		(len(dims) == 2 && dims[0] == voiceRows && dims[1] == styleDim)) {
		return nil, fmt.Errorf("unexpected voice shape %v", dims)
	}
	data := make([]float32, voiceRows*styleDim)
	var word [4]byte
	for i := range data {
		if _, err = io.ReadFull(r, word[:]); err != nil {
			return nil, fmt.Errorf("read float %d: %w", i, err)
		}
		data[i] = math.Float32frombits(binary.LittleEndian.Uint32(word[:]))
	}
	return data, nil
}
