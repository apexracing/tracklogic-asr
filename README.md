# tracklogic-asr

基于 SenseVoiceSmall INT8 和 ONNX Runtime 的离线 Go ASR 库。公开 API 位于根包 `asr`；模型与运行库准备位于 `assets` 包；音频和推理实现通过 `internal` 隔离。

Windows x64 CPU 版 `onnxruntime.dll` 已通过 `go:embed` 内嵌。约 230 MB 的模型在首次使用时从 Hugging Face 下载、校验并缓存，无需 Python、PyTorch 或 FunASR。

## 快速测试

```powershell
go run ./cmd/tracklogic-asr testdata/zh.wav
```

识别本地 WAV：

```powershell
go run ./cmd/tracklogic-asr "D:\Recordings\my-audio.wav"
```

使用 Windows 默认麦克风录音 5 秒并识别：

```powershell
go run ./cmd/tracklogic-asr -record 5s
```

提前准备离线资源：

```powershell
.\scripts\fetch-assets.ps1
```

## Go API

```go
package main

import (
    "context"
    "fmt"
    "log"

    asr "github.com/apexracing/tracklogic-asr"
)

func main() {
    ctx := context.Background()
    recognizer, err := asr.New(ctx, asr.Config{NumThreads: 4})
    if err != nil {
        log.Fatal(err)
    }
    defer recognizer.Close()

    result, err := recognizer.TranscribeFile(ctx, `testdata\zh.wav`, asr.Options{
        Language: asr.LanguageAuto,
    })
    if err != nil {
        log.Fatal(err)
    }
    fmt.Printf("%s [%s, %s, %v]\n", result.Text, result.Language, result.Emotion, result.Events)
}
```

内存中的单声道 `float32` PCM 可直接识别：

```go
result, err := recognizer.Transcribe(ctx, samples, sampleRate, asr.Options{})
```

支持 `auto`、`zh`、`en`、`yue`、`ja`、`ko` 和 `nospeech` 语言提示。默认开启 ITN 和标点；设置 `WithoutITN` 可关闭。

## 资源管理

需要单独预取或使用自定义资源时，使用 `assets` 包：

```go
import "github.com/apexracing/tracklogic-asr/assets"

paths, err := assets.Prepare(ctx, assets.Config{
    ModelDir:    `D:\ASR\sensevoice-small-int8`,
    RuntimePath: `D:\ASR\onnxruntime.dll`,
})
```

`assets.Config` 支持 `ModelCacheDir`、`RuntimeCacheDir` 和 `Progress`。也可分别调用 `assets.EnsureModel` 与 `assets.EnsureRuntime`。

## 目录结构

```text
doc.go, recognizer.go, types.go   package asr 公共识别 API
assets/                           模型下载、缓存和内嵌运行库
internal/audio/                   WAV、混音和重采样
internal/sensevoice/              FBank/LFR/CMVN、CTC、ONNX engine
cmd/tracklogic-asr/               WAV 与麦克风 CLI
cmd/fetch-assets/                 资源预取工具
testdata/                         小型端到端测试音频
```

## 环境和格式

- Windows x64，Go 1.24.3 或更高版本，并启用 CGO。
- WAV 支持 PCM 8/16/24/32-bit 和 IEEE float32。
- 多声道自动混为单声道，其他采样率自动重采样到 16 kHz。
- 当前为离线整段识别，不包含 VAD 和流式分段。

## 许可

项目源码使用 MIT License。内嵌 ONNX Runtime 1.26.0 使用 Microsoft MIT License。SenseVoice 模型权重适用 FunASR Model License；使用和分发模型时需保留 SenseVoice/FunASR 名称及归属信息。
