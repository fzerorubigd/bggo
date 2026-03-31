package bggo

import (
	"context"
	"encoding/xml"
	"fmt"
	"net/http"
)

const userPath = "xmlapi2/user"

// GetUserRequest is the request for the GetUser API.
type GetUserRequest struct {
	Username string
}

// User is a BGG user profile.
type User struct {
	ID             int64  `json:"id"`
	Username       string `json:"username"`
	FirstName      string `json:"first_name,omitempty"`
	LastName       string `json:"last_name,omitempty"`
	AvatarLink     string `json:"avatar_link,omitempty"`
	YearRegistered int    `json:"year_registered"`
	LastLogin      string `json:"last_login,omitempty"`
	State          string `json:"state,omitempty"`
	Country        string `json:"country,omitempty"`
}

// GetUser fetches a user profile by username.
func (c *Client) GetUser(ctx context.Context, req GetUserRequest) (*User, error) {
	u := c.buildURL(userPath, map[string]string{"name": req.Username})

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	resp, err := c.do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("http call: %w", err)
	}
	defer resp.Body.Close()

	var raw xmlUser
	if err = decodeXML(resp.Body, &raw); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return &User{
		ID:             raw.ID,
		Username:       raw.Name,
		FirstName:      raw.FirstName.Value,
		LastName:       raw.LastName.Value,
		AvatarLink:     raw.AvatarLink.Value,
		YearRegistered: int(safeInt(raw.YearRegistered.Value)),
		LastLogin:      raw.LastLogin.Value,
		State:          raw.State.Value,
		Country:        raw.Country.Value,
	}, nil
}

// --- XML structures (private) ---

type xmlUser struct {
	XMLName        xml.Name        `xml:"user"`
	ID             int64           `xml:"id,attr"`
	Name           string          `xml:"name,attr"`
	FirstName      xmlSimpleString `xml:"firstname"`
	LastName       xmlSimpleString `xml:"lastname"`
	AvatarLink     xmlSimpleString `xml:"avatarlink"`
	YearRegistered xmlSimpleString `xml:"yearregistered"`
	LastLogin      xmlSimpleString `xml:"lastlogin"`
	State          xmlSimpleString `xml:"stateorprovince"`
	Country        xmlSimpleString `xml:"country"`
}
