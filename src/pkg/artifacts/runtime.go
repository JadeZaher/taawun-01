package artifacts

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"

	"taawun/web"
)

// bundledDatastarRuntime returns the exact vendored runtime that signed bundles serve.
func bundledDatastarRuntime() ([]byte, error) {
	runtime, err := web.FS.ReadFile(datastarRuntimeBundlePath)
	if err != nil {
		return nil, fmt.Errorf("read pinned Datastar runtime: %w", err)
	}
	// Normalize checkout line endings so the bundle retains the upstream digest.
	runtime = bytes.ReplaceAll(runtime, []byte("\r\n"), []byte("\n"))
	digest := sha256.Sum256(runtime)
	if actual := hex.EncodeToString(digest[:]); actual != datastarRuntimeSHA256 {
		return nil, fmt.Errorf("pinned Datastar runtime digest = %s, want %s", actual, datastarRuntimeSHA256)
	}
	return runtime, nil
}
