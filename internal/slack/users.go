package slack

import (
	"fmt"
	"strconv"
	"strings"
)

// User represents a Slack user summary.
type User struct {
	ID          string `json:"id"`
	Name        string `json:"name,omitempty"`
	RealName    string `json:"real_name,omitempty"`
	DisplayName string `json:"display_name,omitempty"`
	Email       string `json:"email,omitempty"`
	IsBot       bool   `json:"is_bot,omitempty"`
	IsAdmin     bool   `json:"is_admin,omitempty"`
	Title       string `json:"title,omitempty"`
	TZ          string `json:"tz,omitempty"`
}

// ListUsers lists workspace users.
func (c *Client) ListUsers(limit int, includeBots bool) ([]User, error) {
	params := map[string]string{
		"limit": strconv.Itoa(min(limit, 200)),
	}
	var all []User
	for {
		resp, err := c.API("users.list", params)
		if err != nil {
			return nil, err
		}
		members := getArray(resp, "members")
		for _, m := range members {
			rec := toRecord(m)
			u := parseUser(rec)
			if !includeBots && u.IsBot {
				continue
			}
			all = append(all, u)
		}
		cursor := getString(resp, "response_metadata", "next_cursor")
		if cursor == "" || len(all) >= limit {
			break
		}
		params["cursor"] = cursor
	}
	if len(all) > limit {
		all = all[:limit]
	}
	return all, nil
}

// GetUser fetches a user by ID or handle.
func (c *Client) GetUser(input string) (*User, error) {
	if IsUserID(input) {
		return c.getUserByID(input)
	}
	// Search by name/handle
	users, err := c.ListUsers(500, true)
	if err != nil {
		return nil, err
	}
	lower := strings.ToLower(strings.TrimPrefix(input, "@"))
	for _, u := range users {
		if strings.ToLower(u.Name) == lower || strings.ToLower(u.DisplayName) == lower {
			return &u, nil
		}
	}
	return nil, fmt.Errorf("user not found: %s", input)
}

// ResolveUserID resolves a handle, email, or ID to a user ID.
func (c *Client) ResolveUserID(input string) (string, error) {
	if IsUserID(input) {
		return input, nil
	}
	// Try email lookup
	if strings.Contains(input, "@") && strings.Contains(input, ".") {
		resp, err := c.API("users.lookupByEmail", map[string]string{
			"email": input,
		})
		if err == nil {
			u := toRecord(resp["user"])
			if id := stringVal(u, "id"); id != "" {
				return id, nil
			}
		}
	}
	u, err := c.GetUser(input)
	if err != nil {
		return "", err
	}
	return u.ID, nil
}

func (c *Client) getUserByID(id string) (*User, error) {
	resp, err := c.API("users.info", map[string]string{
		"user": id,
	})
	if err != nil {
		return nil, err
	}
	rec := toRecord(resp["user"])
	u := parseUser(rec)
	return &u, nil
}

func parseUser(rec map[string]any) User {
	profile := toRecord(rec["profile"])
	u := User{
		ID:       stringVal(rec, "id"),
		Name:     stringVal(rec, "name"),
		RealName: stringVal(rec, "real_name"),
		TZ:       stringVal(rec, "tz"),
	}
	if profile != nil {
		u.DisplayName = stringVal(profile, "display_name")
		u.Email = stringVal(profile, "email")
		u.Title = stringVal(profile, "title")
	}
	if v, ok := rec["is_bot"].(bool); ok {
		u.IsBot = v
	}
	if v, ok := rec["is_admin"].(bool); ok {
		u.IsAdmin = v
	}
	return u
}
