package auth

import (
	"os/exec"
	"runtime"
	"strings"
)

// KeychainGet reads a password from macOS keychain.
func KeychainGet(account, service string) string {
	if runtime.GOOS != "darwin" {
		return ""
	}
	out, err := exec.Command("security", "find-generic-password", "-s", service, "-a", account, "-w").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// KeychainSet stores a password in macOS keychain.
func KeychainSet(account, value, service string) bool {
	if runtime.GOOS != "darwin" {
		return false
	}
	// Delete first (ignore error)
	_ = exec.Command("security", "delete-generic-password", "-s", service, "-a", account).Run()
	err := exec.Command("security", "add-generic-password", "-s", service, "-a", account, "-w", value).Run()
	return err == nil
}
