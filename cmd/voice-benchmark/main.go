package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"sort"
	"strings"
	"time"

	voice "github.com/apexracing/tracklogic-voice"
	"github.com/apexracing/tracklogic-voice/assets"
	"github.com/apexracing/tracklogic-voice/internal/audio"
	"github.com/apexracing/tracklogic-voice/internal/processmetrics"
)

const (
	warmupCount        = 3
	measureCount       = 10
	goMaxProcs         = 2
	defaultONNXThreads = 2
	regressionPct      = 15.0
)

type report struct {
	Metadata       metadata           `json:"metadata"`
	Initialization map[string]float64 `json:"session_initialization_ms"`
	Scenarios      []scenario         `json:"scenarios"`
}

type metadata struct {
	CPU               string `json:"cpu"`
	LogicalCPUs       int    `json:"logical_cpus"`
	OS                string `json:"os"`
	Architecture      string `json:"architecture"`
	GoVersion         string `json:"go_version"`
	ONNXRuntime       string `json:"onnx_runtime"`
	ASRRevision       string `json:"asr_revision"`
	TTSModelScopeRev  string `json:"tts_modelscope_revision"`
	TTSHuggingFaceRev string `json:"tts_huggingface_revision"`
	Commit            string `json:"commit"`
	TestedAt          string `json:"tested_at"`
	GOMAXPROCS        int    `json:"gomaxprocs"`
	IntraOpThreads    int    `json:"onnx_intra_op_threads"`
	InterOpThreads    int    `json:"onnx_inter_op_threads"`
	ExecutionMode     string `json:"execution_mode"`
	Warmups           int    `json:"warmups"`
	Measurements      int    `json:"measurements"`
}

type scenario struct {
	Name                 string  `json:"name"`
	FirstInferenceMS     float64 `json:"first_inference_ms"`
	MeanMS               float64 `json:"mean_ms"`
	P50MS                float64 `json:"p50_ms"`
	P95MS                float64 `json:"p95_ms"`
	MinMS                float64 `json:"min_ms"`
	MaxMS                float64 `json:"max_ms"`
	RealTimeFactor       float64 `json:"real_time_factor"`
	OpsPerSecond         float64 `json:"ops_per_second"`
	CPUTimeMS            float64 `json:"cpu_time_ms"`
	CPUPercentSingleCore float64 `json:"cpu_percent_single_core"`
	CPUPercentMachine    float64 `json:"cpu_percent_machine"`
	WorkingSetBytes      uint64  `json:"working_set_bytes"`
	PeakWorkingSetBytes  uint64  `json:"peak_working_set_bytes"`
	PrivateBytes         uint64  `json:"private_bytes"`
	GoHeapAllocBytes     uint64  `json:"go_heap_alloc_bytes"`
	GoHeapSysBytes       uint64  `json:"go_heap_sys_bytes"`
	GoTotalAllocBytes    uint64  `json:"go_total_alloc_bytes"`
	GoGCs                uint32  `json:"go_gc_count"`
	GoNSPerOp            uint64  `json:"go_ns_per_op"`
	GoBytesPerOp         uint64  `json:"go_bytes_per_op"`
	GoAllocsPerOp        uint64  `json:"go_allocs_per_op"`
	MediaDurationSeconds float64 `json:"media_duration_seconds"`
}

func main() {
	var (
		asrDir      = flag.String("asr-model-dir", "", "prepared SenseVoice model directory")
		ttsDir      = flag.String("tts-model-dir", "", "prepared Kokoro model directory")
		asrCacheDir = flag.String("asr-model-cache-dir", "", "ASR model download directory when asr-model-dir is empty")
		ttsCacheDir = flag.String("tts-model-cache-dir", "", "TTS model download directory when tts-model-dir is empty")
		source      = flag.String("model-source", "modelscope", "modelscope or huggingface")
		wav         = flag.String("wav", filepath.FromSlash("testdata/zh.wav"), "ASR benchmark WAV")
		output      = flag.String("out", filepath.FromSlash("benchmark-results/baseline-windows-i7-10700f.json"), "JSON report path")
		compare     = flag.String("compare", "", "optional baseline JSON to compare with a 15% threshold")
		cpuName     = flag.String("cpu", "Intel Core i7-10700F (8 cores, 16 threads)", "CPU description stored in the report")
		onnxThreads = flag.Int("onnx-threads", defaultONNXThreads, "ONNX intra-op CPU threads")
	)
	flag.Parse()
	if runtime.GOOS != "windows" || runtime.GOARCH != "amd64" {
		fatalf("benchmark runner requires windows/amd64")
	}
	if *onnxThreads <= 0 {
		fatalf("onnx-threads must be greater than zero")
	}
	runtime.GOMAXPROCS(goMaxProcs)
	ctx := context.Background()
	selectedSource := assets.ModelSource(*source)

	// Asset preparation is intentionally outside every initialization and
	// inference timer.
	asrAssets, err := assets.PrepareASR(ctx, assets.ASRConfig{
		ModelDir: *asrDir, ModelCacheDir: *asrCacheDir, ModelSource: selectedSource, Progress: progress,
	})
	if err != nil {
		fatalf("prepare ASR assets: %v", err)
	}
	ttsAssets, err := assets.PrepareTTS(ctx, assets.TTSConfig{
		ModelDir: *ttsDir, ModelCacheDir: *ttsCacheDir, ModelSource: selectedSource, Progress: progress,
	})
	if err != nil {
		fatalf("prepare TTS assets: %v", err)
	}
	samples, sampleRate, err := audio.ReadWAV(*wav)
	if err != nil {
		fatalf("read benchmark WAV: %v", err)
	}
	asrMediaSeconds := float64(len(samples)) / float64(sampleRate)

	r := report{
		Metadata: metadata{
			CPU: *cpuName, LogicalCPUs: runtime.NumCPU(), OS: runtime.GOOS, Architecture: runtime.GOARCH,
			GoVersion: runtime.Version(), ONNXRuntime: assets.ONNXRuntimeVersion,
			ASRRevision: assets.ModelScopeModelRevision, TTSModelScopeRev: assets.ModelScopeTTSRevision,
			TTSHuggingFaceRev: assets.HuggingFaceTTSRevision, Commit: gitCommit(), TestedAt: time.Now().Format(time.RFC3339),
			GOMAXPROCS: goMaxProcs, IntraOpThreads: *onnxThreads, InterOpThreads: 1, ExecutionMode: "sequential",
			Warmups: warmupCount, Measurements: measureCount,
		},
		Initialization: map[string]float64{},
	}

	started := time.Now()
	recognizer, err := voice.NewRecognizer(ctx, voice.RecognizerConfig{Assets: assets.ASRConfig{
		ModelDir: asrAssets.Model.Directory, RuntimePath: asrAssets.RuntimePath,
	}, NumThreads: *onnxThreads})
	if err != nil {
		fatalf("initialize recognizer: %v", err)
	}
	r.Initialization["asr"] = milliseconds(time.Since(started))
	asrScenario, err := measure("asr", asrMediaSeconds, func() (float64, error) {
		_, inferErr := recognizer.Transcribe(ctx, samples, sampleRate, voice.TranscriptionOptions{Language: voice.LanguageChinese})
		return asrMediaSeconds, inferErr
	})
	if closeErr := recognizer.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		fatalf("benchmark ASR: %v", err)
	}
	r.Scenarios = append(r.Scenarios, asrScenario)

	started = time.Now()
	synthesizer, err := voice.NewSynthesizer(ctx, voice.SynthesizerConfig{Assets: assets.TTSConfig{
		ModelDir: ttsAssets.Model.Directory, RuntimePath: ttsAssets.RuntimePath,
	}, NumThreads: *onnxThreads})
	if err != nil {
		fatalf("initialize synthesizer: %v", err)
	}
	r.Initialization["tts"] = milliseconds(time.Since(started))
	inputs := []struct {
		name string
		text string
	}{
		{"tts_zh", "千里之行，始于足下。"},
		{"tts_en", "Tracklogic Voice provides clear, efficient speech synthesis for practical applications."},
		{"tts_mixed", "Tracklogic Voice 支持中文、English 和中英混合语音合成。"},
	}
	for _, input := range inputs {
		result, measureErr := measure(input.name, 0, func() (float64, error) {
			waveform, synthErr := synthesizer.Synthesize(ctx, input.text, voice.SynthesisOptions{TrimSilence: true})
			return float64(len(waveform)) / voice.SynthesisSampleRate, synthErr
		})
		if measureErr != nil {
			_ = synthesizer.Close()
			fatalf("benchmark %s: %v", input.name, measureErr)
		}
		r.Scenarios = append(r.Scenarios, result)
	}
	if err = synthesizer.Close(); err != nil {
		fatalf("close synthesizer: %v", err)
	}

	if err = writeReport(*output, r); err != nil {
		fatalf("write report: %v", err)
	}
	if *compare != "" {
		if err = compareReports(*compare, r); err != nil {
			fatalf("regression: %v", err)
		}
	}
	fmt.Println(*output)
}

func measure(name string, fixedMediaSeconds float64, operation func() (float64, error)) (scenario, error) {
	started := time.Now()
	mediaSeconds, err := operation()
	first := time.Since(started)
	if err != nil {
		return scenario{}, err
	}
	for range warmupCount {
		if _, err = operation(); err != nil {
			return scenario{}, err
		}
	}
	var beforeGo, afterGo runtime.MemStats
	runtime.ReadMemStats(&beforeGo)
	beforeProcess, err := processmetrics.Read()
	if err != nil {
		return scenario{}, err
	}
	durations := make([]time.Duration, measureCount)
	var measuredWall time.Duration
	for i := range durations {
		started = time.Now()
		currentMediaSeconds, operationErr := operation()
		durations[i] = time.Since(started)
		if operationErr != nil {
			return scenario{}, operationErr
		}
		if fixedMediaSeconds == 0 {
			mediaSeconds = currentMediaSeconds
		}
		measuredWall += durations[i]
	}
	afterProcess, err := processmetrics.Read()
	if err != nil {
		return scenario{}, err
	}
	runtime.ReadMemStats(&afterGo)
	sorted := append([]time.Duration(nil), durations...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })
	mean := measuredWall / measureCount
	cpu := afterProcess.CPUTime - beforeProcess.CPUTime
	allocBytes := afterGo.TotalAlloc - beforeGo.TotalAlloc
	allocs := afterGo.Mallocs - beforeGo.Mallocs
	return scenario{
		Name: name, FirstInferenceMS: milliseconds(first), MeanMS: milliseconds(mean),
		P50MS: milliseconds(percentile(sorted, 0.50)), P95MS: milliseconds(percentile(sorted, 0.95)),
		MinMS: milliseconds(sorted[0]), MaxMS: milliseconds(sorted[len(sorted)-1]),
		RealTimeFactor: mean.Seconds() / mediaSeconds, OpsPerSecond: 1 / mean.Seconds(),
		CPUTimeMS: milliseconds(cpu), CPUPercentSingleCore: float64(cpu) / float64(measuredWall) * 100,
		CPUPercentMachine: float64(cpu) / float64(measuredWall) * 100 / float64(runtime.NumCPU()),
		WorkingSetBytes:   afterProcess.WorkingSet, PeakWorkingSetBytes: afterProcess.PeakWorkingSet,
		PrivateBytes: afterProcess.PrivateBytes, GoHeapAllocBytes: afterGo.HeapAlloc, GoHeapSysBytes: afterGo.HeapSys,
		GoTotalAllocBytes: allocBytes, GoGCs: afterGo.NumGC - beforeGo.NumGC,
		GoNSPerOp: uint64(mean.Nanoseconds()), GoBytesPerOp: allocBytes / measureCount, GoAllocsPerOp: allocs / measureCount,
		MediaDurationSeconds: mediaSeconds,
	}, nil
}

func percentile(values []time.Duration, p float64) time.Duration {
	index := int(math.Ceil(float64(len(values))*p)) - 1
	return values[max(0, min(index, len(values)-1))]
}

func milliseconds(d time.Duration) float64 { return float64(d) / float64(time.Millisecond) }

func writeReport(path string, value report) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	if err = os.WriteFile(path, data, 0o644); err != nil {
		return err
	}
	markdownPath := strings.TrimSuffix(path, filepath.Ext(path)) + ".md"
	return os.WriteFile(markdownPath, []byte(markdown(value)), 0o644)
}

func markdown(r report) string {
	var b strings.Builder
	fmt.Fprintln(&b, "# tracklogic-voice CPU baseline")
	fmt.Fprintf(&b, "\n- Tested: %s\n- CPU: %s\n- OS: %s/%s\n- Go: %s\n- ONNX Runtime: %s CPU\n- Commit: `%s`\n- Threads: GOMAXPROCS=%d, intra-op=%d, inter-op=%d, sequential\n- Runs: %d warmups, %d measurements\n",
		r.Metadata.TestedAt, r.Metadata.CPU, r.Metadata.OS, r.Metadata.Architecture, r.Metadata.GoVersion,
		r.Metadata.ONNXRuntime, r.Metadata.Commit, r.Metadata.GOMAXPROCS, r.Metadata.IntraOpThreads,
		r.Metadata.InterOpThreads, r.Metadata.Warmups, r.Metadata.Measurements)
	fmt.Fprintln(&b, "\n| Scenario | First ms | Mean ms | p50 ms | p95 ms | RTF | ops/s | CPU single-core | CPU machine | Peak WS MiB | Private MiB | ns/op | B/op | allocs/op |")
	fmt.Fprintln(&b, "|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|")
	for _, s := range r.Scenarios {
		fmt.Fprintf(&b, "| %s | %.2f | %.2f | %.2f | %.2f | %.3f | %.3f | %.1f%% | %.1f%% | %.1f | %.1f | %d | %d | %d |\n",
			s.Name, s.FirstInferenceMS, s.MeanMS, s.P50MS, s.P95MS, s.RealTimeFactor, s.OpsPerSecond,
			s.CPUPercentSingleCore, s.CPUPercentMachine, mib(s.PeakWorkingSetBytes), mib(s.PrivateBytes),
			s.GoNSPerOp, s.GoBytesPerOp, s.GoAllocsPerOp)
	}
	fmt.Fprintln(&b, "\nSession initialization (asset download excluded):")
	fmt.Fprintf(&b, "\n- ASR: %.2f ms\n- TTS: %.2f ms\n", r.Initialization["asr"], r.Initialization["tts"])
	fmt.Fprintln(&b, "\nCompare a later run with this baseline:")
	fmt.Fprintln(&b, "\n```powershell\ngo run ./cmd/voice-benchmark -asr-model-cache-dir C:\\\\your-app\\\\models\\\\sensevoice-small-int8 -tts-model-cache-dir C:\\\\your-app\\\\models\\\\kokoro-82m-v1.1-zh -compare benchmark-results/baseline-windows-i7-10700f.json -out benchmark-results/current.json\n```")
	fmt.Fprintln(&b, "\nThe command fails when mean latency, CPU time per operation, or peak working set regresses by more than 15%.")
	return b.String()
}

func compareReports(path string, current report) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var baseline report
	if err = json.Unmarshal(data, &baseline); err != nil {
		return err
	}
	base := map[string]scenario{}
	for _, s := range baseline.Scenarios {
		base[s.Name] = s
	}
	var failures []string
	for _, s := range current.Scenarios {
		old, ok := base[s.Name]
		if !ok {
			continue
		}
		checks := []struct {
			name         string
			old, current float64
		}{
			{"mean latency", old.MeanMS, s.MeanMS},
			{"CPU time/op", old.CPUTimeMS / measureCount, s.CPUTimeMS / measureCount},
			{"peak working set", float64(old.PeakWorkingSetBytes), float64(s.PeakWorkingSetBytes)},
		}
		for _, check := range checks {
			if check.old > 0 && (check.current/check.old-1)*100 > regressionPct {
				failures = append(failures, fmt.Sprintf("%s %s increased from %.2f to %.2f", s.Name, check.name, check.old, check.current))
			}
		}
	}
	if len(failures) > 0 {
		return fmt.Errorf("%s", strings.Join(failures, "; "))
	}
	return nil
}

func mib(bytes uint64) float64 { return float64(bytes) / (1024 * 1024) }

func gitCommit() string {
	cmd := exec.Command("git", "rev-parse", "HEAD")
	output, err := cmd.Output()
	if err != nil {
		if info, ok := debug.ReadBuildInfo(); ok {
			for _, setting := range info.Settings {
				if setting.Key == "vcs.revision" {
					return setting.Value
				}
			}
		}
		return "unknown"
	}
	revision := strings.TrimSpace(string(output))
	status := exec.Command("git", "status", "--porcelain")
	if dirty, statusErr := status.Output(); statusErr == nil && len(dirty) > 0 {
		revision += "-dirty"
	}
	return revision
}

func progress(name string, completed, total int64) {
	if total > 0 {
		fmt.Fprintf(os.Stderr, "\r%-35s %3d%%", name, completed*100/total)
		if completed == total {
			fmt.Fprintln(os.Stderr)
		}
	}
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
