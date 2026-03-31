package bggo

import (
	"context"
	"encoding/xml"
	"fmt"
	"net/http"
)

const personPath = "xmlapi2/person"

// GetPersonRequest is the request for the GetPerson API.
type GetPersonRequest struct {
	ID int64
}

// Person is a BGG person (designer, artist, publisher, etc.).
type Person struct {
	ID        int64    `json:"id"`
	Type      ItemType `json:"type"`
	Thumbnail string   `json:"thumbnail,omitempty"`
	Image     string   `json:"image,omitempty"`
}

// GetPerson fetches a person by their BGG ID.
func (c *Client) GetPerson(ctx context.Context, req GetPersonRequest) (*Person, error) {
	u := c.buildURL(personPath, map[string]string{
		"id": fmt.Sprint(req.ID),
	})

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	resp, err := c.do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("http call: %w", err)
	}
	defer resp.Body.Close()

	var raw xmlPersonItems
	if err = decodeXML(resp.Body, &raw); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return &Person{
		ID:        safeInt(raw.Item.ID),
		Type:      ItemType(raw.Item.Type),
		Thumbnail: raw.Item.Thumbnail,
		Image:     raw.Item.Image,
	}, nil
}

// --- XML structures (private) ---

type xmlPersonItems struct {
	XMLName xml.Name `xml:"items"`
	Item    struct {
		Type      string `xml:"type,attr"`
		ID        string `xml:"id,attr"`
		Thumbnail string `xml:"thumbnail"`
		Image     string `xml:"image"`
	} `xml:"item"`
}
