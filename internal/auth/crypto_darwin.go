package auth

import (
	"os/exec"
	"strings"
)

func getSafeStoragePasswordsOS(prefix string) []string {
	_ = prefix
	queries := []struct {
		service string
		account string
	}{
		{"Slack Safe Storage", "Slack Key"},
		{"Slack Safe Storage", "Slack App Store Key"},
		{"Slack Safe Storage", ""},
		{"Chrome Safe Storage", ""},
		{"Chromium Safe Storage", ""},
	}
	var passwords []string
	for _, q := range queries {
		args := []string{"find-generic-password", "-w", "-s", q.service}
		if q.account != "" {
			args = append(args, "-a", q.account)
		}
		out, err := exec.Command("security", args...).Output()
		if err != nil {
			continue
		}
		if s := strings.TrimSpace(string(out)); s != "" {
			passwords = append(passwords, s)
		}
	}
	return passwords
}
