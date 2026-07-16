# tracklogic-voice

`tracklogic-voice` 是面向 Windows x64 上层应用的离线语音库，提供统一的 Go API：

- SenseVoice INT8 自动语音识别（ASR）
- Kokoro 82M FP32 中文、美式英语及中英混合语音合成（TTS）
- 内嵌 ONNX Runtime 1.26.0 CPU 运行库
- 固定模型版本、SHA-256 校验和 ModelScope/Hugging Face 下载源

运行时不依赖 Python、PyTorch、Misaki 或 eSpeak NG。库不包含麦克风采集或录音功能。

## 环境要求

- Windows x64
- Go 1.24.3 或更高版本
- CGO 已启用

```powershell
go get github.com/apexracing/tracklogic-voice
```

## 资源目录

模型目录由上层应用管理。调用方必须选择以下一种方式：

1. 通过 `ModelDir` 提供已经准备好的模型目录；
2. 通过 `ModelCacheDir` 指定下载和长期保存模型的目录。

库不会为模型选择系统临时目录或默认缓存目录。`ModelSource` 未设置时使用 ModelScope，也可以显式指定 `assets.ModelSourceHuggingFace`。下载内容固定到已校验的版本，失败时不会自动切换来源。

`RuntimePath` 可以留空，此时使用内嵌的 ONNX Runtime；如需控制运行库释放位置，可设置 `RuntimeCacheDir`。

## ASR

```go
package main

import (
	"context"
	"log"
	"path/filepath"

	voice "github.com/apexracing/tracklogic-voice"
	"github.com/apexracing/tracklogic-voice/assets"
)

func recognize(ctx context.Context, modelRoot, wavPath string) {
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

	result, err := recognizer.TranscribeFile(ctx, wavPath, voice.TranscriptionOptions{
		Language: voice.LanguageAuto,
	})
	if err != nil {
		log.Fatal(err)
	}
	log.Println(result.Text)
}
```

`Transcribe` 接受单声道 `[]float32` PCM 和对应采样率，非 16 kHz 输入会自动重采样。`TranscribeFile` 支持未压缩 WAV，包括 PCM 8/16/24/32-bit、IEEE float32 和多声道自动混音。

## TTS

```go
func synthesize(ctx context.Context, modelRoot, outputPath string) {
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

	err = synthesizer.SynthesizeFile(ctx, "赛车工程师报告：前方3号弯，注意刹车温度。", outputPath, voice.SynthesisOptions{
		Voice:       "zm_009",
		Language:    voice.LanguageAuto,
		Speed:       1,
		TrimSilence: true,
	})
	if err != nil {
		log.Fatal(err)
	}
}
```

`Synthesize` 和 `SynthesizePhonemes` 返回 24 kHz 单声道 `[]float32` PCM；`SynthesizeFile` 输出 16-bit PCM WAV。纯英文默认声线为 `af_maple`，中文和中英混合文本默认声线为 `zf_001`。

当前提供给上层应用的精选声线：

| Voice ID | 显示名称 | 语言 | 性别 |
|---|---|---|---|
| `zf_022` | 晴岚 | 中文 | 女声 |
| `zf_001` | 若溪 | 中文 | 女声 |
| `zf_046` | 凌音 | 中文 | 女声 |
| `zm_058` | 长风 | 中文 | 男声 |
| `zm_069` | 观澜 | 中文 | 男声 |
| `zm_009` | 知衡 | 中文 | 男声 |
| `af_maple` | Maple | 美式英语 | 女声 |
| `bf_vale` | Vale | 英式英语 | 女声 |

使用 `voice.SelectableVoices()` 获取可直接导出到上层应用的结构化列表。`Synthesizer.Voices()` 返回模型中全部原始 voice ID。

## 命令行工具

仓库提供用于开发验证和提前准备资源的统一命令，不提供录音命令：

```powershell
go run ./cmd/tracklogic-voice fetch -model-cache-dir .\models\sensevoice-small-int8 asr
go run ./cmd/tracklogic-voice fetch -model-cache-dir .\models\kokoro-82m-v1.1-zh tts

go run ./cmd/tracklogic-voice asr -model-cache-dir .\models\sensevoice-small-int8 testdata\zh.wav
go run ./cmd/tracklogic-voice tts -model-cache-dir .\models\kokoro-82m-v1.1-zh -out voice.wav "你好，Tracklogic Voice!"
```

## 模型版本

TTS 资源从以下固定版本下载，两处内容按 SHA-256 校验：

- ModelScope：`huntsman/voice`，revision `49e8fe2437ce5cc1c9af35d3285e56b14980099f`
- Hugging Face：`tracklogic/voice`，revision `7b789e1fc5e3cf09e4634dc53c7fc4ed062ab00b`

TTS 模型路径保留为 `onnx/model_quantized.onnx` 以兼容已有目录结构，但当前文件内容为 **FP32**，SHA-256 为 `94b973941b1852754f979be5d5e20be666d5c81d9bb886b88ae1dc85c9b895ca`。FP16 及其他实验模型不在当前支持范围内。

## 测试与性能验证

```powershell
go test ./...
go test -race ./...
go vet ./...
go build ./cmd/...
```

需要本地模型的端到端测试会在资源不存在时自动跳过。可通过 `SENSEVOICE_MODEL_DIR`、`SENSEVOICE_TEST_WAV`、`KOKORO_MODEL_DIR` 和 `ONNXRUNTIME_DLL` 指定测试资源。

性能 runner 默认使用 ONNX intra-op 2 线程、inter-op 1 线程、`GOMAXPROCS=2`，执行 3 次预热和 10 次测量：

```powershell
go run ./cmd/voice-benchmark `
  -asr-model-cache-dir .\models\sensevoice-small-int8 `
  -tts-model-cache-dir .\models\kokoro-82m-v1.1-zh `
  -out .\benchmark-results\current.json
```

可通过 `-onnx-threads` 调整 ONNX intra-op 线程数。性能结果与本机硬件、系统负载和模型版本强相关，因此 `benchmark-results` 仅作为本地输出目录，不提交到仓库。

## 兼容性与限制

- 当前仅支持 CPU 推理和离线整段 ASR/TTS，不提供流式接口或 GPU 后端。
- `New`、`Config`、`Options`、`Result` 和 `assets.Prepare` 仅为源代码兼容保留，已标记弃用。
- `Recognizer` 和 `Synthesizer` 持有原生资源，使用完成后必须调用 `Close`。

## 许可

项目源码使用 MIT License。内嵌 ONNX Runtime 使用 Microsoft MIT License；CMUdict 许可见 [THIRD_PARTY_NOTICES.md](THIRD_PARTY_NOTICES.md)。模型权重适用对应模型仓库公布的许可与归属要求。
