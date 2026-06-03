package auth

import (
	"crypto/aes"
	"crypto/cipher"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"unsafe"

	"golang.org/x/sys/windows"
)

func getSafeStoragePasswordsOS(_ string, _ []string, _ []string) []string {
	// Windows uses DPAPI, not password-based encryption.
	return nil
}

// DecryptCookieWindows decrypts a Chromium v10/v11 cookie on Windows.
//
// Layout (Chromium v80+): "v10"/"v11" prefix (3 bytes) + nonce (12 bytes) +
// ciphertext + GCM auth tag (16 bytes), encrypted with AES-256-GCM. The AES key
// is stored DPAPI-wrapped in "<localStateDir>/Local State" under
// os_crypt.encrypted_key (base64, "DPAPI" prefix).
func DecryptCookieWindows(encrypted []byte, localStateDir string) (string, error) {
	key, err := chromiumMasterKey(localStateDir)
	if err != nil {
		return "", err
	}

	// 3-byte version prefix + 12-byte nonce + ciphertext + 16-byte tag.
	const prefixLen, nonceLen, tagLen = 3, 12, 16
	if len(encrypted) < prefixLen+nonceLen+tagLen {
		return "", fmt.Errorf("encrypted cookie too short (%d bytes)", len(encrypted))
	}
	nonce := encrypted[prefixLen : prefixLen+nonceLen]
	ciphertext := encrypted[prefixLen+nonceLen:] // ciphertext || tag; gcm.Open splits the trailing tag

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	plain, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", fmt.Errorf("AES-256-GCM decrypt failed: %w", err)
	}
	return string(plain), nil
}

// chromiumMasterKey reads and DPAPI-decrypts the AES master key from a
// Chromium "Local State" file.
func chromiumMasterKey(localStateDir string) ([]byte, error) {
	localStatePath := filepath.Join(localStateDir, "Local State")
	raw, err := os.ReadFile(localStatePath)
	if err != nil {
		return nil, fmt.Errorf("read Local State (%s): %w", localStatePath, err)
	}
	var ls struct {
		OSCrypt struct {
			EncryptedKey string `json:"encrypted_key"`
		} `json:"os_crypt"`
	}
	if err := json.Unmarshal(raw, &ls); err != nil {
		return nil, fmt.Errorf("parse Local State: %w", err)
	}
	if ls.OSCrypt.EncryptedKey == "" {
		return nil, fmt.Errorf("no os_crypt.encrypted_key in Local State")
	}
	blob, err := base64.StdEncoding.DecodeString(ls.OSCrypt.EncryptedKey)
	if err != nil {
		return nil, fmt.Errorf("decode encrypted_key: %w", err)
	}
	if len(blob) < 5 || string(blob[:5]) != "DPAPI" {
		return nil, fmt.Errorf("encrypted_key missing DPAPI prefix")
	}
	return dpapiUnprotect(blob[5:])
}

// dpapiUnprotect decrypts a DPAPI-protected blob for the current user via
// CryptUnprotectData (crypt32.dll). No CGO required.
func dpapiUnprotect(data []byte) ([]byte, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("empty DPAPI blob")
	}
	in := windows.DataBlob{Size: uint32(len(data)), Data: &data[0]}
	var out windows.DataBlob
	if err := windows.CryptUnprotectData(&in, nil, nil, 0, nil, 0, &out); err != nil {
		return nil, fmt.Errorf("CryptUnprotectData: %w", err)
	}
	defer windows.LocalFree(windows.Handle(unsafe.Pointer(out.Data))) //nolint:errcheck
	res := make([]byte, out.Size)
	copy(res, unsafe.Slice(out.Data, out.Size))
	return res, nil
}
