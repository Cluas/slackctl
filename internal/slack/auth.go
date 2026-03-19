package slack

// Auth represents Slack authentication credentials.
// Two modes: standard bot/user token, or browser-extracted xoxc+xoxd.
type Auth struct {
	Type      AuthType `json:"auth_type"`
	Token     string   `json:"token,omitempty"`      // standard token (xoxb-/xoxp-)
	XoxcToken string   `json:"xoxc_token,omitempty"` // browser token
	XoxdCookie string  `json:"xoxd_cookie,omitempty"` // browser cookie
}

type AuthType string

const (
	AuthStandard AuthType = "standard"
	AuthBrowser  AuthType = "browser"
)
