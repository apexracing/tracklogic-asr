# tracklogic-asr

直接使用 ONNX Runtime 运行 SenseVoiceSmall INT8 模型的 Go ASR 库。无需 Python、PyTorch 或 FunASR。Windows x64 CPU 运行库已通过 `go:embed` 内嵌，模型在首次使用时自动下载、校验并缓存。

## 快速测试

识别仓库自带的中文 WAV：

```powershell
go run ./cmd/sensevoice testdata/zh.wav
```

首次运行只需下载约 230 MB 的 INT8 模型；内嵌的 ONNX Runtime DLL 会校验后释放到 `%LOCALAPPDATA%\tracklogic-asr`。之后全部复用缓存。

识别任意本地 WAV：

```powershell
go run ./cmd/sensevoice "D:\Recordings\my-audio.wav"
```

使用 Windows 默认麦克风录音 5 秒并立即识别：

```powershell
go run ./cmd/sensevoice -record 5s
```

录音默认保存为 `testdata/local-recording.wav`。也可以指定保存位置：

```powershell
go run ./cmd/sensevoice -record 10s -record-out "D:\Recordings\speech.wav"
```

## 提前准备离线资源

联网环境下运行一次：

```powershell
.\scripts\fetch-assets.ps1
```

脚本会下载并验证模型，同时释放和验证内嵌的 ONNX Runtime。缓存完成后，识别过程不再需要联网。

## Go 库用法

```go
package main

import (
    "context"
    "fmt"
    "log"

    sensevoice "github.com/apexracing/tracklogic-asr"
)

func main() {
    ctx := context.Background()
    recognizer, err := sensevoice.New(ctx, sensevoice.Config{
        NumThreads: 4,
        Progress: func(name string, downloaded, total int64) {
            if total > 0 {
                fmt.Printf("\r%s %.1f%%", name, float64(downloaded)*100/float64(total))
            }
        },
    })
    if err != nil {
        log.Fatal(err)
    }
    defer recognizer.Close()

    result, err := recognizer.TranscribeFile(ctx, `testdata\zh.wav`, sensevoice.Options{
        Language: sensevoice.LanguageAuto,
    })
    if err != nil {
        log.Fatal(err)
    }

    fmt.Println(result.Text)
    fmt.Printf("language=%s emotion=%s events=%v\n",
        result.Language, result.Emotion, result.Events)
}
```

内存中的单声道 `float32` PCM 可以直接识别，采样率不必预先转换成 16 kHz：

```go
result, err := recognizer.Transcribe(ctx, samples, sampleRate, sensevoice.Options{})
```

默认开启 ITN 和标点。设置 `Options.WithoutITN = true` 可以关闭。支持 `auto`、`zh`、`en`、`yue`、`ja`、`ko` 和 `nospeech` 语言提示。

## 自定义或私有部署

可以覆盖模型目录或内嵌运行库：

```go
recognizer, err := sensevoice.New(ctx, sensevoice.Config{
    RuntimePath: `D:\ASR\onnxruntime.dll`,
    ModelDir:    `D:\ASR\sensevoice-small-int8`,
})
```

模型目录需包含 `model_quant.onnx`、`tokens.json`、`am.mvn` 和 `config.yaml`。

`Config.CacheDir` 和 `Config.RuntimeCacheDir` 可修改默认缓存位置。模型固定 revision 并校验每个文件；内嵌 DLL 也会校验 SHA-256。所有临时文件验证成功后才原子安装。

## 项目结构

```text
cmd/sensevoice/          WAV 与 Windows 麦克风完整示例
cmd/fetch-assets/        模型预下载和内嵌运行库释放工具
scripts/fetch-assets.ps1 一键离线资源准备脚本
testdata/zh.wav          小型端到端中文测试音频
runtime/windows-amd64/   通过 go:embed 内嵌的 DLL 与许可
```

约 230 MB 的模型不提交到 Go module，避免其他项目执行 `go get` 时下载数百 MB 或遇到 Git LFS 指针文件。约 15 MB 的 DLL 随 Go module 分发，使 Windows 用户无需单独安装 ONNX Runtime。

## 环境和格式

- Windows x64。
- Go 1.24.3 或更高版本，并启用 CGO。
- WAV 支持 PCM 8/16/24/32-bit 和 IEEE float32。
- 多声道自动混为单声道，其他采样率自动重采样到 16 kHz。
- 当前为离线整段识别，不包含 VAD 和流式分段。

## 许可

项目源码使用 MIT License。内嵌的 ONNX Runtime 1.26.0 使用 Microsoft MIT License，许可文本随 DLL 一起分发。

SenseVoice 模型权重适用 FunASR Model License。使用和分发模型时需要保留 SenseVoice/FunASR 名称及归属信息，并确认目标用途满足模型许可。
