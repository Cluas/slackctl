package auth

import (
	"os/exec"
	"strings"
)

func getSafeStoragePasswordsOS(prefix string) []string {
	attributes := [][]string{
		{"application", "com.slack.Slack"},
		{"application", "Slack"},
		{"application", "slack"},
		{"service", "Slack Safe Storage"},
	}
	var passwords []string
	for _, pair := range attributes {
		out, err := exec.Command("secret-tool", append([]string{"lookup"}, pair...)...).Output()
		if err != nil {
			continue
		}
		if s := strings.TrimSpace(string(out)); s != "" {
			passwords = append(passwords, s)
		}
	}
	// Chromium Linux OSCrypt v10 fallback
	if prefix == "v11" {
		passwords = append(passwords, "")
	}
	passwords = append(passwords, "peanuts")
	return passwords
}
