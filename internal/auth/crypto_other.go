//go:build !windows

package auth

import "fmt"

// DecryptCookieWindows is a no-op on non-Windows platforms; it only exists so
// callers guarded by runtime.GOOS == "windows" compile across all targets. The
// real DPAPI + AES-256-GCM implementation is in crypto_windows.go.
func DecryptCookieWindows(_ []byte, _ string) (string, error) {
	return "", fmt.Errorf("DPAPI decryption is only available on Windows")
}
