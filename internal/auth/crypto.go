package auth

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha1"
	"fmt"
	"regexp"

	"golang.org/x/crypto/pbkdf2"
)

// DecryptChromiumCookie decrypts a Chromium cookie value using PBKDF2 + AES-128-CBC.
func DecryptChromiumCookie(data []byte, password string, iterations int) (string, error) {
	if len(data) == 0 {
		return "", nil
	}
	if iterations < 1 {
		return "", fmt.Errorf("iterations must be >= 1, got %d", iterations)
	}

	salt := []byte("saltysalt")
	iv := []byte("                ") // 16 spaces
	key := pbkdf2.Key([]byte(password), salt, iterations, 16, sha1.New)

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}

	if len(data)%aes.BlockSize != 0 {
		return "", fmt.Errorf("ciphertext is not a multiple of block size")
	}

	mode := cipher.NewCBCDecrypter(block, iv)
	plain := make([]byte, len(data))
	mode.CryptBlocks(plain, data)

	// Remove PKCS7 padding
	if len(plain) > 0 {
		padLen := int(plain[len(plain)-1])
		if padLen > 0 && padLen <= aes.BlockSize && padLen <= len(plain) {
			plain = plain[:len(plain)-padLen]
		}
	}

	// Find xoxd- token in decrypted text
	xoxdRe := regexp.MustCompile(`xoxd-[A-Za-z0-9%/+_=.-]+`)
	match := xoxdRe.FindString(string(plain))
	if match != "" {
		return match, nil
	}
	return string(plain), nil
}

// GetSafeStoragePasswords retrieves Chromium Safe Storage passwords for the
// Slack/Chrome built-in defaults.
func GetSafeStoragePasswords(prefix string) []string {
	return dedupeStrings(getSafeStoragePasswordsOS(prefix, nil, nil))
}

// GetChromiumSafeStoragePasswords retrieves Safe Storage passwords for a specific
// Chromium-family browser (e.g. Brave). macServices are extra macOS keychain
// "Safe Storage" service names to query; linuxApps are extra Linux secret-tool
// "application" attribute values to query. Both are tried in addition to the
// built-in Slack/Chromium defaults.
func GetChromiumSafeStoragePasswords(prefix string, macServices, linuxApps []string) []string {
	return dedupeStrings(getSafeStoragePasswordsOS(prefix, macServices, linuxApps))
}

func dedupeStrings(in []string) []string {
	seen := make(map[string]bool)
	var unique []string
	for _, p := range in {
		if !seen[p] {
			seen[p] = true
			unique = append(unique, p)
		}
	}
	return unique
}

// DecryptCookieWindows decrypts a Chromium v10/v11 cookie on Windows using
// DPAPI + AES-256-GCM. The real implementation lives in crypto_windows.go;
// crypto_other.go provides a stub for non-Windows builds. localStateDir is the
// directory containing the browser's "Local State" file (the Slack data dir, or
// the Chrome "User Data" dir).

