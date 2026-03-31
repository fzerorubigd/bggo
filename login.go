package bggo

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

const loginPath = "login/api/v1"

// LoginRequest is the request for the Login API.
type LoginRequest struct {
	Username string
	Password string
}

// Login authenticates with BGG and stores the session cookies on the client
// for subsequent authenticated requests.
func (c *Client) Login(ctx context.Context, req LoginRequest) error {
	payload := map[string]any{
		"credentials": map[string]string{
			"username": req.Username,
			"password": req.Password,
		},
	}

	b, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal payload: %w", err)
	}

	u := c.buildURL(loginPath, nil)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, u, bytes.NewReader(b))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.do(httpReq)
	if err != nil {
		return fmt.Errorf("http call: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("login failed: %s", resp.Status)
	}

	c.lock.Lock()
	defer c.lock.Unlock()

	c.cookies = resp.Cookies()
	c.username = req.Username

	return nil
}
