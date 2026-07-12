package sensevoice

import (
	"bufio"
	"fmt"
	"math"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/mjibson/go-dsp/fft"
)

const (
	targetSampleRate = 16000
	melBins          = 80
	frameLength      = 400
	frameShift       = 160
	lfrWindow        = 7
	lfrShift         = 6
	featureDim       = melBins * lfrWindow
)

type frontend struct {
	shift []float32
	scale []float32
	bank  [melBins][]melWeight
}

type melWeight struct {
	bin    int
	weight float64
}

func newFrontend(cmvnPath string) (*frontend, error) {
	shift, scale, err := loadCMVN(cmvnPath)
	if err != nil {
		return nil, err
	}
	if len(shift) < featureDim || len(scale) < featureDim {
		return nil, fmt.Errorf("CMVN has %d/%d values, need %d", len(shift), len(scale), featureDim)
	}
	f := &frontend{shift: shift[:featureDim], scale: scale[:featureDim]}
	f.initMelBank()
	return f, nil
}

func (f *frontend) extract(samples []float32, sampleRate int) ([]float32, int, error) {
	if sampleRate <= 0 {
		return nil, 0, fmt.Errorf("sample rate must be positive")
	}
	if len(samples) < sampleRate/10 {
		return nil, 0, fmt.Errorf("audio is too short: need at least 100 ms")
	}
	samples = resampleLinear(samples, sampleRate, targetSampleRate)
	frames := 1 + (len(samples)-frameLength)/frameShift
	if frames <= 0 {
		return nil, 0, fmt.Errorf("audio is too short for feature extraction")
	}
	fbank := make([]float32, frames*melBins)
	windowed := make([]float64, 512)
	for t := 0; t < frames; t++ {
		frame := samples[t*frameShift : t*frameShift+frameLength]
		var mean float64
		for _, v := range frame {
			mean += float64(v) * 32768
		}
		mean /= frameLength
		for i := range windowed {
			windowed[i] = 0
		}
		var prev float64
		for i, v := range frame {
			x := float64(v)*32768 - mean
			y := x - 0.97*prev
			if i == 0 {
				y = x * (1 - 0.97)
			}
			prev = x
			windowed[i] = y * (0.54 - 0.46*math.Cos(2*math.Pi*float64(i)/float64(frameLength-1)))
		}
		spectrum := fft.FFTReal(windowed)
		power := make([]float64, 257)
		for i := range power {
			power[i] = real(spectrum[i])*real(spectrum[i]) + imag(spectrum[i])*imag(spectrum[i])
		}
		for m, weights := range f.bank {
			var energy float64
			for _, w := range weights {
				energy += power[w.bin] * w.weight
			}
			if energy < 1.1920928955078125e-07 {
				energy = 1.1920928955078125e-07
			}
			fbank[t*melBins+m] = float32(math.Log(energy))
		}
	}

	lfrFrames := (frames + lfrShift - 1) / lfrShift
	out := make([]float32, lfrFrames*featureDim)
	for t := 0; t < lfrFrames; t++ {
		start := t*lfrShift - (lfrWindow-1)/2
		for w := 0; w < lfrWindow; w++ {
			src := start + w
			if src < 0 {
				src = 0
			} else if src >= frames {
				src = frames - 1
			}
			copy(out[t*featureDim+w*melBins:], fbank[src*melBins:(src+1)*melBins])
		}
		for d := 0; d < featureDim; d++ {
			idx := t*featureDim + d
			out[idx] = (out[idx] + f.shift[d]) * f.scale[d]
		}
	}
	return out, lfrFrames, nil
}

func (f *frontend) initMelBank() {
	mel := func(hz float64) float64 { return 1127 * math.Log1p(hz/700) }
	low, high := mel(20), mel(targetSampleRate/2)
	points := make([]float64, melBins+2)
	for i := range points {
		points[i] = low + (high-low)*float64(i)/float64(melBins+1)
	}
	for m := 0; m < melBins; m++ {
		left, center, right := points[m], points[m+1], points[m+2]
		for bin := 0; bin <= 256; bin++ {
			binMel := mel(float64(bin) * targetSampleRate / 512)
			var weight float64
			if binMel > left && binMel < center {
				weight = (binMel - left) / (center - left)
			} else if binMel >= center && binMel < right {
				weight = (right - binMel) / (right - center)
			}
			if weight > 0 {
				f.bank[m] = append(f.bank[m], melWeight{bin, weight})
			}
		}
	}
}

var bracketValues = regexp.MustCompile(`\[([^\]]+)\]`)

func loadCMVN(path string) ([]float32, []float32, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, nil, fmt.Errorf("open CMVN: %w", err)
	}
	defer f.Close()
	var shift, scale []float32
	scanner := bufio.NewScanner(f)
	mode := ""
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "<AddShift>") {
			mode = "shift"
			continue
		}
		if strings.HasPrefix(line, "<Rescale>") {
			mode = "scale"
			continue
		}
		if !strings.Contains(line, "<LearnRateCoef>") || mode == "" {
			continue
		}
		match := bracketValues.FindStringSubmatch(line)
		if len(match) != 2 {
			continue
		}
		values := make([]float32, 0, featureDim)
		for _, field := range strings.Fields(match[1]) {
			v, e := strconv.ParseFloat(field, 32)
			if e != nil {
				return nil, nil, fmt.Errorf("parse CMVN value %q: %w", field, e)
			}
			values = append(values, float32(v))
		}
		if mode == "shift" {
			shift = values
		} else {
			scale = values
		}
	}
	if err = scanner.Err(); err != nil {
		return nil, nil, err
	}
	return shift, scale, nil
}
