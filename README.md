# tracklogic-voice

纯 Go 的离线语音库，同时提供 SenseVoice ASR 和 Kokoro 中英文 TTS。公开 API 位于根包 `voice`，推理使用内嵌的 ONNX Runtime 1.26.0 CPU 版；运行时不依赖 Python、PyTorch、Misaki 或 eSpeak NG。

当前支持 Windows x64。ASR 接受多种采样率的 WAV 或内存 PCM；TTS 支持中文、美式英语和自动分段的中英混合文本，输出 24 kHz 单声道 float32 PCM 或 16-bit PCM WAV。

## 快速使用

识别 WAV：

```powershell
go run ./cmd/tracklogic-voice asr -model-cache-dir .\models\sensevoice-small-int8 testdata/zh.wav
```

合成中文、英文或混合文本：

```powershell
go run ./cmd/tracklogic-voice tts -model-cache-dir .\models\kokoro-82m-v1.1-zh -out voice.wav "你好，Tracklogic Voice!"
```

模型下载目录必须由上层应用通过 `ModelCacheDir`（CLI 中为 `-model-cache-dir`）显式指定。首次运行会从 ModelScope 下载并校验固定版本的模型；显式使用 Hugging Face 时加上 `-model-source huggingface`，下载失败不会自动切换来源。

提前准备离线资源也统一使用 `tracklogic-voice fetch`：

```powershell
go run ./cmd/tracklogic-voice fetch -model-cache-dir .\models\sensevoice-small-int8 asr
go run ./cmd/tracklogic-voice fetch -model-cache-dir .\models\kokoro-82m-v1.1-zh tts
```

## Go API

```go
package main

import (
	"context"
	"log"
	"path/filepath"

	voice "github.com/apexracing/tracklogic-voice"
	"github.com/apexracing/tracklogic-voice/assets"
)

func main() {
	ctx := context.Background()
	modelRoot := `D:\Tracklogic\models` // 由上层应用决定

	recognizer, err := voice.NewRecognizer(ctx, voice.RecognizerConfig{
		Assets: assets.ASRConfig{
			ModelCacheDir: filepath.Join(modelRoot, "sensevoice-small-int8"),
			ModelSource:   assets.ModelSourceModelScope,
		},
		NumThreads: 2,
	})
	if err != nil {
		log.Fatal(err)
	}
	defer recognizer.Close()
	result, err := recognizer.TranscribeFile(ctx, `testdata\zh.wav`, voice.TranscriptionOptions{
		Language: voice.LanguageAuto,
	})
	if err != nil {
		log.Fatal(err)
	}
	log.Println(result.Text)

	synthesizer, err := voice.NewSynthesizer(ctx, voice.SynthesizerConfig{
		Assets: assets.TTSConfig{
			ModelCacheDir: filepath.Join(modelRoot, "kokoro-82m-v1.1-zh"),
			ModelSource:   assets.ModelSourceModelScope,
		},
		NumThreads: 2,
	})
	if err != nil {
		log.Fatal(err)
	}
	defer synthesizer.Close()
	if err = synthesizer.SynthesizeFile(ctx, "千里之行，始于足下。", "voice.wav", voice.SynthesisOptions{
		TrimSilence: true,
	}); err != nil {
		log.Fatal(err)
	}
}
```

`Synthesize` 和 `SynthesizePhonemes` 返回 24 kHz 单声道 `[]float32`。纯英文默认 voice 为 `af_maple`，中文和混合文本默认 `zf_001`；`Voices` 返回聚合包中的 103 个 voice。`SynthesisOptions.Language` 可设为 `auto`、`zh` 或 `en`。

上层应用可通过 `voice.SelectableVoices()` 获取筛选后的 8 个可选声线及其显示名、语言地区和性别；`Synthesizer.Voices()` 仍返回模型内全部原始 voice ID：

```go
for _, option := range voice.SelectableVoices() {
	log.Printf("%s: %s (%s, %s)", option.ID, option.Name, option.Locale, option.Gender)
}
```

旧版 `New`、`Config`、`Options`、`Result` 与 `assets.Prepare` 仍可编译，但已标记弃用。

## G2P

中文前端使用 `gse v1.0.2`、`go-pinyin v0.21.0`、数字规范化、变调、轻声和儿化规则。英文前端内嵌固定 revision `74790861f652b15e4ac49015a90074ad62a27690` 的 CMUdict；词典未命中的单词由纯 Go letter-to-sound 规则处理。混合文本按 Unicode 文字类型分段后合并音素。

## 模型资源

TTS 的两个固定下载源提供字节一致的资源：

- ModelScope：`huntsman/voice`，revision `49e8fe2437ce5cc1c9af35d3285e56b14980099f`
- Hugging Face：`tracklogic/voice`，revision `7b789e1fc5e3cf09e4634dc53c7fc4ed062ab00b`

固定校验 `onnx/model_quantized.onnx`、`voices/voices-v1.1-zh.bin`、`tokenizer.json`、`tokenizer_config.json` 和 `config.json`。ModelScope LFS 下载仅使用 GET。未提供现成 `ModelDir` 时，`assets.PrepareASR` 和 `assets.PrepareTTS` 要求上层应用提供 `ModelCacheDir`，库不会自行选择系统缓存目录；`RuntimePath` 仍可留空以使用内嵌运行库。

`onnx/model_quantized.onnx` 文件名为兼容现有目录结构而保留，当前固定内容是 FP32 模型，SHA-256 为 `94b973941b1852754f979be5d5e20be666d5c81d9bb886b88ae1dc85c9b895ca`。

## CPU 性能测试

独立 runner 固定 `GOMAXPROCS=2`、ONNX inter-op=1、sequential execution，ONNX intra-op 默认为 2，并运行 3 次预热和 10 次正式测量。可通过 `-onnx-threads` 单独调整 ONNX intra-op 线程数：

```powershell
go run ./cmd/voice-benchmark `
  -asr-model-cache-dir .\models\sensevoice-small-int8 `
  -tts-model-cache-dir .\models\kokoro-82m-v1.1-zh
```

它覆盖 ASR、中文 TTS、英文 TTS 和混合 TTS，输出 JSON 与 Markdown，记录初始化、首次推理、mean/p50/p95/min/max、RTF、ops/s、CPU 时间和占用、Working Set、Private Bytes 及 Go 内存指标。标准 Go benchmarks：

```powershell
$env:TRACKLOGIC_BENCH='1'
$env:SENSEVOICE_MODEL_DIR='.\models\sensevoice-small-int8'
$env:KOKORO_MODEL_DIR='.\models\kokoro-82m-v1.1-zh'
go test -run '^$' -bench '2Threads$' -benchmem .
```

将新结果与已提交 baseline 比较，延迟、CPU 时间或峰值内存超过 15% 会返回失败：

```powershell
go run ./cmd/voice-benchmark `
  -asr-model-cache-dir .\models\sensevoice-small-int8 `
  -tts-model-cache-dir .\models\kokoro-82m-v1.1-zh `
  -compare benchmark-results/baseline-windows-i7-10700f.json `
  -out benchmark-results/current.json
```

## 环境与许可

- Windows x64、Go 1.24.3 或更高版本，并启用 CGO。
- ASR WAV 支持 PCM 8/16/24/32-bit 和 IEEE float32；多声道自动混音，采样率自动转换到 16 kHz。
- 当前为离线整段 ASR/TTS，不包含流式接口或 GPU 后端。

项目源码使用 MIT License。内嵌 ONNX Runtime 使用 Microsoft MIT License；CMUdict 许可见 [THIRD_PARTY_NOTICES.md](THIRD_PARTY_NOTICES.md)。模型权重适用各自模型仓库公布的许可与归属要求。
