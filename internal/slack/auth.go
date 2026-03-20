package slack

// Auth represents Slack authentication credentials.
// Two modes: standard bot/user token, or browser-extracted xoxc+xoxd.
type Auth struct {
	Type          AuthType   `json:"auth_type"`
	Source        AuthSource `json:"auth_source,omitempty"`    // where credentials were extracted from
	Token         string     `json:"token,omitempty"`          // standard token (xoxb-/xoxp-)
	XoxcToken     string     `json:"xoxc_token,omitempty"`     // browser token
	XoxdCookie    string     `json:"xoxd_cookie,omitempty"`    // browser cookie
	EnterpriseURL string     `json:"enterprise_url,omitempty"` // Enterprise Grid org URL for API routing
}

type AuthType string

const (
	AuthStandard AuthType = "standard"
	AuthBrowser  AuthType = "browser"
)

// AuthSource indicates where the credentials were extracted from.
type AuthSource string

const (
	SourceDesktop AuthSource = "desktop"
	SourceChrome  AuthSource = "chrome"
	SourceBrave   AuthSource = "brave"
	SourceFirefox AuthSource = "firefox"
	SourceManual  AuthSource = "manual" // auth add, parse-curl, env vars
)
