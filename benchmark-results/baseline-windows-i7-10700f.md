# tracklogic-voice CPU baseline

- Tested: 2026-07-16T19:08:36+08:00
- CPU: Intel Core i7-10700F (8 cores, 16 threads)
- OS: windows/amd64
- Go: go1.25.3
- ONNX Runtime: 1.26.0 CPU
- Commit: `1fac6edf27149f6e85b58adc9ab985433c955c64-dirty`
- Threads: GOMAXPROCS=2, intra-op=2, inter-op=1, sequential
- Runs: 3 warmups, 10 measurements

| Scenario | First ms | Mean ms | p50 ms | p95 ms | RTF | ops/s | CPU single-core | CPU machine | Peak WS MiB | Private MiB | ns/op | B/op | allocs/op |
|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|
| asr | 249.90 | 240.11 | 235.18 | 255.85 | 0.043 | 4.165 | 203.7% | 12.7% | 352.2 | 367.9 | 240112970 | 27195997 | 15667 |
| tts_zh | 3913.34 | 2606.55 | 2600.84 | 2640.55 | 1.419 | 0.384 | 195.0% | 12.2% | 623.1 | 652.7 | 2606545520 | 908652 | 193 |
| tts_en | 5808.85 | 5521.06 | 5507.24 | 5587.27 | 1.152 | 0.181 | 178.2% | 11.1% | 770.8 | 815.6 | 5521062040 | 2046871 | 173 |
| tts_mixed | 5307.07 | 5139.78 | 5130.57 | 5220.69 | 1.200 | 0.195 | 179.3% | 11.2% | 783.8 | 829.3 | 5139780610 | 1872212 | 353 |

Session initialization (asset download excluded):

- ASR: 3162.27 ms
- TTS: 1474.10 ms

Compare a later run with this baseline:

```powershell
go run ./cmd/voice-benchmark -compare benchmark-results/baseline-windows-i7-10700f.json -out benchmark-results/current.json
```

The command fails when mean latency, CPU time per operation, or peak working set regresses by more than 15%.
