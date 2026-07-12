//go:build windows && amd64

package assets

import _ "embed"

//go:embed bundled/windows-amd64/onnxruntime.dll
var embeddedRuntimeDLL []byte

//go:embed bundled/windows-amd64/LICENSE-onnxruntime.txt
var embeddedRuntimeLicense []byte
