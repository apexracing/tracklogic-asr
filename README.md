# tracklogic-voice

纯 Go 的离线语音库，同时提供 SenseVoice ASR 和 Kokoro 中英文 TTS。公开 API 位于根包 `voice`，推理使用内嵌的 ONNX Runtime 1.26.0 CPU 版；运行时不依赖 Python、PyTorch、Misaki 或 eSpeak NG。

当前支持 Windows x64。ASR 接受多种采样率的 WAV 或内存 PCM；TTS 支持中文、美式英语和自动分段的中英混合文本，输出 24 kHz 单声道 float32 PCM 或 16-bit PCM WAV。

## 快速使用

识别 WAV：

```powershell
go run ./cmd/tracklogic-voice asr testdata/zh.wav
```

合成中文、英文或混合文本：

```powershell
go run ./cmd/tracklogic-voice tts -out voice.wav "你好，Tracklogic Voice!"
```

首次运行会从 ModelScope 下载并校验固定版本的模型。显式使用 Hugging Face 时加上 `-model-source huggingface`；下载失败不会自动切换来源。

提前准备全部离线资源：

```powershell
.\scripts\fetch-assets.ps1
```

## Go API

```go
package main

import (
	"context"
	"log"

	voice "github.com/apexracing/tracklogic-voice"
	"github.com/apexracing/tracklogic-voice/assets"
)

func main() {
	ctx := context.Background()

	recognizer, err := voice.NewRecognizer(ctx, voice.RecognizerConfig{
		Assets: assets.ASRConfig{ModelSource: assets.ModelSourceModelScope},
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
		Assets: assets.TTSConfig{ModelSource: assets.ModelSourceModelScope},
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

旧版 `New`、`Config`、`Options`、`Result` 与 `assets.Prepare` 仍可编译，但已标记弃用。

## G2P

中文前端使用 `gse v1.0.2`、`go-pinyin v0.21.0`、数字规范化、变调、轻声和儿化规则。英文前端内嵌固定 revision `74790861f652b15e4ac49015a90074ad62a27690` 的 CMUdict；词典未命中的单词由纯 Go letter-to-sound 规则处理。混合文本按 Unicode 文字类型分段后合并音素。

## 模型资源

TTS 的两个固定下载源共享同一份 `manifest-v1` 缓存：

- ModelScope：`huntsman/Kokoro-82M-v1.1-zh-ONNX`，revision `80d48ea07e671ec7de0f3f59c32941d3a5c00e53`
- Hugging Face：`tracklogic/Kokoro-82M-v1.1-zh-ONNX`，revision `f78bc7bcc7b3646cbf899829ca31cc5c852bbb31`

固定校验 `onnx/model_quantized.onnx`、`voices/voices-v1.1-zh.bin`、`tokenizer.json`、`tokenizer_config.json` 和 `config.json`。ModelScope LFS 下载仅使用 GET。可通过 `assets.PrepareASR`、`assets.PrepareTTS` 或自定义 `ModelDir`/`RuntimePath` 管理资源。

## CPU 性能测试

独立 runner 固定 `GOMAXPROCS=2`、ONNX intra-op=2、inter-op=1、sequential execution，并运行 3 次预热和 10 次正式测量：

```powershell
go run ./cmd/voice-benchmark -tts-model-dir .\Kokoro-82M-v1.1-zh-ONNX
```

它覆盖 ASR、中文 TTS、英文 TTS 和混合 TTS，输出 JSON 与 Markdown，记录初始化、首次推理、mean/p50/p95/min/max、RTF、ops/s、CPU 时间和占用、Working Set、Private Bytes 及 Go 内存指标。标准 Go benchmarks：

```powershell
$env:TRACKLOGIC_BENCH='1'
go test -run '^$' -bench '2Threads$' -benchmem .
```

将新结果与已提交 baseline 比较，延迟、CPU 时间或峰值内存超过 15% 会返回失败：

```powershell
go run ./cmd/voice-benchmark -compare benchmark-results/baseline-windows-i7-10700f.json -out benchmark-results/current.json
```

## 环境与许可

- Windows x64、Go 1.24.3 或更高版本，并启用 CGO。
- ASR WAV 支持 PCM 8/16/24/32-bit 和 IEEE float32；多声道自动混音，采样率自动转换到 16 kHz。
- 当前为离线整段 ASR/TTS，不包含流式接口或 GPU 后端。

项目源码使用 MIT License。内嵌 ONNX Runtime 使用 Microsoft MIT License；CMUdict 许可见 [THIRD_PARTY_NOTICES.md](THIRD_PARTY_NOTICES.md)。模型权重适用各自模型仓库公布的许可与归属要求。
