package audio

import (
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"os"
)

// ReadWAV reads an uncompressed PCM or IEEE-float WAV file and mixes channels
// down to mono. Resampling to 16 kHz is performed by Recognizer.Transcribe.
func ReadWAV(path string) ([]float32, int, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, 0, fmt.Errorf("open wav: %w", err)
	}
	defer f.Close()

	var header [12]byte
	if _, err = io.ReadFull(f, header[:]); err != nil || string(header[:4]) != "RIFF" || string(header[8:]) != "WAVE" {
		return nil, 0, fmt.Errorf("invalid RIFF/WAVE file")
	}
	var format, channels, bits uint16
	var sampleRate uint32
	var data []byte
	for {
		var ch [8]byte
		if _, err = io.ReadFull(f, ch[:]); err != nil {
			break
		}
		size := binary.LittleEndian.Uint32(ch[4:])
		switch string(ch[:4]) {
		case "fmt ":
			b := make([]byte, size)
			if _, err = io.ReadFull(f, b); err != nil || len(b) < 16 {
				return nil, 0, fmt.Errorf("read wav format chunk")
			}
			format = binary.LittleEndian.Uint16(b[0:2])
			channels = binary.LittleEndian.Uint16(b[2:4])
			sampleRate = binary.LittleEndian.Uint32(b[4:8])
			bits = binary.LittleEndian.Uint16(b[14:16])
		case "data":
			data = make([]byte, size)
			if _, err = io.ReadFull(f, data); err != nil {
				return nil, 0, fmt.Errorf("read wav data: %w", err)
			}
		default:
			if _, err = f.Seek(int64(size), io.SeekCurrent); err != nil {
				return nil, 0, err
			}
		}
		if size%2 == 1 {
			_, _ = f.Seek(1, io.SeekCurrent)
		}
		if len(data) > 0 && format != 0 {
			break
		}
	}
	if channels == 0 || sampleRate == 0 || len(data) == 0 {
		return nil, 0, fmt.Errorf("wav is missing format or audio data")
	}
	bytesPerSample := int(bits / 8)
	if bytesPerSample == 0 || len(data)%(bytesPerSample*int(channels)) != 0 {
		return nil, 0, fmt.Errorf("invalid wav sample layout")
	}
	frames := len(data) / (bytesPerSample * int(channels))
	out := make([]float32, frames)
	for i := 0; i < frames; i++ {
		var sum float64
		for c := 0; c < int(channels); c++ {
			off := (i*int(channels) + c) * bytesPerSample
			v, e := decodeSample(data[off:off+bytesPerSample], format, bits)
			if e != nil {
				return nil, 0, e
			}
			sum += float64(v)
		}
		out[i] = float32(sum / float64(channels))
	}
	return out, int(sampleRate), nil
}

func decodeSample(b []byte, format, bits uint16) (float32, error) {
	if format == 3 && bits == 32 {
		return math.Float32frombits(binary.LittleEndian.Uint32(b)), nil
	}
	if format != 1 {
		return 0, fmt.Errorf("unsupported wav format %d", format)
	}
	switch bits {
	case 8:
		return (float32(b[0]) - 128) / 128, nil
	case 16:
		return float32(int16(binary.LittleEndian.Uint16(b))) / 32768, nil
	case 24:
		v := int32(b[0]) | int32(b[1])<<8 | int32(b[2])<<16
		if v&0x800000 != 0 {
			v |= ^0xffffff
		}
		return float32(v) / 8388608, nil
	case 32:
		return float32(int32(binary.LittleEndian.Uint32(b))) / 2147483648, nil
	default:
		return 0, fmt.Errorf("unsupported PCM bit depth %d", bits)
	}
}

func ResampleLinear(in []float32, from, to int) []float32 {
	if from == to || len(in) == 0 {
		return append([]float32(nil), in...)
	}
	n := int(math.Round(float64(len(in)) * float64(to) / float64(from)))
	out := make([]float32, n)
	step := float64(from) / float64(to)
	for i := range out {
		pos := float64(i) * step
		j := int(pos)
		if j >= len(in)-1 {
			out[i] = in[len(in)-1]
			continue
		}
		frac := float32(pos - float64(j))
		out[i] = in[j] + (in[j+1]-in[j])*frac
	}
	return out
}
