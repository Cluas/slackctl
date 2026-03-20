package auth

import (
	"fmt"
	"net/url"
	"regexp"
)

// ParsedCurlTokens holds tokens extracted from a cURL command.
type ParsedCurlTokens struct {
	WorkspaceURL string
	XoxcToken    string
	XoxdCookie   string
}

var (
	curlURLRe     = regexp.MustCompile(`curl\s+['"]?(https?://([^'"\s]+\.slack\.com)[^'"\s]*)`)
	curlCookieRe  = regexp.MustCompile(`(?:-b|--cookie)\s+\$?'([^']+)'|(?:-b|--cookie)\s+\$?"([^"]+)"|-H\s+\$?'[Cc]ookie:\s*([^']+)'|-H\s+\$?"[Cc]ookie:\s*([^"]+)"`)
	xoxdCookieRe  = regexp.MustCompile(`(?:^|;\s*)d=(xoxd-[^;]+)`)
	tokenPatterns = []*regexp.Regexp{
		regexp.MustCompile(`(?:^|[?&\s])token=(xoxc-[A-Za-z0-9-]+)`),
		regexp.MustCompile(`"token"\s*:\s*"(xoxc-[A-Za-z0-9-]+)"`),
		regexp.MustCompile(`name="token"[^x]*?(xoxc-[A-Za-z0-9-]+)`),
		regexp.MustCompile(`\b(xoxc-[A-Za-z0-9-]+)\b`),
	}
)

// ParseSlackCurlCommand extracts tokens from a cURL command string.
func ParseSlackCurlCommand(curlInput string) (*ParsedCurlTokens, error) {
	urlMatch := curlURLRe.FindStringSubmatch(curlInput)
	if urlMatch == nil {
		return nil, fmt.Errorf("could not find Slack workspace URL in cURL command")
	}
	workspaceURL := "https://" + urlMatch[2]

	cookieMatch := curlCookieRe.FindStringSubmatch(curlInput)
	cookieHeader := ""
	if cookieMatch != nil {
		for _, s := range cookieMatch[1:] {
			if s != "" {
				cookieHeader = s
				break
			}
		}
	}
	// Try cookie header first, then fall back to searching the whole command
	xoxdMatch := xoxdCookieRe.FindStringSubmatch(cookieHeader)
	if xoxdMatch == nil {
		xoxdMatch = xoxdCookieRe.FindStringSubmatch(curlInput)
	}
	if xoxdMatch == nil {
		return nil, fmt.Errorf("could not find xoxd cookie (d=xoxd-...) in cURL command")
	}
	xoxdCookie, err := url.QueryUnescape(xoxdMatch[1])
	if err != nil {
		xoxdCookie = xoxdMatch[1]
	}

	var xoxcToken string
	for _, re := range tokenPatterns {
		m := re.FindStringSubmatch(curlInput)
		if m != nil && len(m) > 1 && m[1] != "" {
			xoxcToken = m[1]
			break
		}
	}
	if xoxcToken == "" {
		return nil, fmt.Errorf("could not find xoxc token in cURL command")
	}

	return &ParsedCurlTokens{
		WorkspaceURL: workspaceURL,
		XoxcToken:    xoxcToken,
		XoxdCookie:   xoxdCookie,
	}, nil
}
