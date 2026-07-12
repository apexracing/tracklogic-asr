//go:build windows && amd64

package sensevoice

import _ "embed"

//go:embed runtime/windows-amd64/onnxruntime.dll
var embeddedRuntimeDLL []byte

//go:embed runtime/windows-amd64/LICENSE-onnxruntime.txt
var embeddedRuntimeLicense []byte
