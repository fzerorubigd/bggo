package bggo

import (
	"context"
	"encoding/xml"
	"fmt"
	"net/http"
	"strings"
)

const searchPath = "xmlapi2/search"

// SearchRequest is the request for the Search API.
type SearchRequest struct {
	Query string
	Types []ItemType
	Exact bool
}

// SearchResult is a single result from the search API.
type SearchResult struct {
	ID             int64    `json:"id"`
	Name           string   `json:"name"`
	AlternateNames []string `json:"alternate_names,omitempty"`
	Type           ItemType `json:"type"`
	YearPublished  int      `json:"year_published"`
}

// Search searches for items on BGG by query string.
func (c *Client) Search(ctx context.Context, req SearchRequest) ([]SearchResult, error) {
	args := map[string]string{
		"query": req.Query,
	}
	if req.Exact {
		args["exact"] = "1"
	}
	if len(req.Types) > 0 {
		types := make([]string, len(req.Types))
		for i, t := range req.Types {
			types[i] = string(t)
		}
		args["type"] = strings.Join(types, ",")
	}

	u := c.buildURL(searchPath, args)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	resp, err := c.do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("http call: %w", err)
	}
	defer resp.Body.Close()

	var raw xmlSearchItems
	if err = decodeXML(resp.Body, &raw); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	results := make([]SearchResult, len(raw.Item))
	for i, item := range raw.Item {
		name, alts := extractNames(item.Name)
		results[i] = SearchResult{
			ID:             item.ID,
			Name:           name,
			AlternateNames: alts,
			Type:           ItemType(item.Type),
			YearPublished:  int(safeInt(item.YearPublished.Value)),
		}
	}

	return results, nil
}

// --- XML structures (private) ---

type xmlSearchItem struct {
	Type          string          `xml:"type,attr"`
	ID            int64           `xml:"id,attr"`
	Name          []xmlNameStruct `xml:"name"`
	YearPublished xmlSimpleString `xml:"yearpublished"`
}

type xmlSearchItems struct {
	XMLName xml.Name        `xml:"items"`
	Item    []xmlSearchItem `xml:"item"`
}
