package bggo

import (
	"context"
	"encoding/xml"
	"fmt"
	"net/http"
	"strings"
	"time"
)

const collectionPath = "xmlapi2/collection"

// CollectionStatus represents a filter/status for collection items.
type CollectionStatus string

const (
	CollectionOwn        CollectionStatus = "own"
	CollectionRated      CollectionStatus = "rated"
	CollectionPlayed     CollectionStatus = "played"
	CollectionComment    CollectionStatus = "comment"
	CollectionTrade      CollectionStatus = "trade"
	CollectionWant       CollectionStatus = "want"
	CollectionWishList   CollectionStatus = "wishlist"
	CollectionPreorder   CollectionStatus = "preorder"
	CollectionWantToPlay CollectionStatus = "wanttoplay"
	CollectionWantToBuy  CollectionStatus = "wanttobuy"
	CollectionPrevOwned  CollectionStatus = "prevowned"
	CollectionHasParts   CollectionStatus = "hasparts"
	CollectionWantParts  CollectionStatus = "wantparts"
)

// WishListPriority values as defined by BGG.
const (
	WishListMustHave        = 1
	WishListLoveToHave      = 2
	WishListLikeToHave      = 3
	WishListThinkingAboutIt = 4
	WishListDoNotBuy        = 5
)

// GetCollectionRequest is the request for the GetCollection API.
type GetCollectionRequest struct {
	Username       string
	IDs            []int64
	CollID         int64
	SubType        ItemType
	ExcludeSubType ItemType
	Statuses       []CollectionStatus
	MinRating      int
	Rating         int
	MinBGGRating   int
	BGGRating      int
	MinPlays       int
	MaxPlays       int
	ModifiedSince  *time.Time
}

func (r *GetCollectionRequest) toParams() map[string]string {
	p := map[string]string{
		"username": r.Username,
	}

	if r.SubType != "" {
		p["subtype"] = string(r.SubType)
	}
	if r.ExcludeSubType != "" {
		p["excludesubtype"] = string(r.ExcludeSubType)
	}
	if r.MinRating > 0 {
		p["minrating"] = fmt.Sprint(r.MinRating)
	}
	if r.Rating > 0 {
		p["rating"] = fmt.Sprint(r.Rating)
	}
	if r.MinBGGRating > 0 {
		p["minbggrating"] = fmt.Sprint(r.MinBGGRating)
	}
	if r.BGGRating > 0 {
		p["bggrating"] = fmt.Sprint(r.BGGRating)
	}
	if r.MinPlays > 0 {
		p["minplays"] = fmt.Sprint(r.MinPlays)
	}
	if r.MaxPlays > 0 {
		p["maxplays"] = fmt.Sprint(r.MaxPlays)
	}
	if r.CollID > 0 {
		p["collid"] = fmt.Sprint(r.CollID)
	}
	if r.ModifiedSince != nil {
		p["modifiedsince"] = r.ModifiedSince.Format("06-01-02")
	}
	for _, s := range r.Statuses {
		p[string(s)] = "1"
	}
	if len(r.IDs) > 0 {
		ids := make([]string, 0, len(r.IDs))
		for _, id := range r.IDs {
			if id > 0 {
				ids = append(ids, fmt.Sprint(id))
			}
		}
		p["id"] = strings.Join(ids, ",")
	}

	return p
}

// CollectionItem is a single item in a user's collection.
type CollectionItem struct {
	ID            int64    `json:"id"`
	CollID        int64    `json:"coll_id"`
	Name          string   `json:"name"`
	Type          ItemType `json:"type"`
	YearPublished int      `json:"year_published"`
	Thumbnail     string   `json:"thumbnail,omitempty"`
	Image         string   `json:"image,omitempty"`
	NumPlays      int      `json:"num_plays"`

	Status []CollectionStatus `json:"status,omitempty"`
}

// GetCollection fetches a user's collection. BGG may return 202 while generating
// the response; this method retries with backoff until the data is ready or the
// context is cancelled.
func (c *Client) GetCollection(ctx context.Context, req GetCollectionRequest) ([]CollectionItem, error) {
	u := c.buildURL(collectionPath, req.toParams())

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	var (
		resp  *http.Response
		delay = time.Second
	)
	for attempt := 1; ; attempt++ {
		resp, err = c.do(httpReq)
		if err != nil {
			return nil, fmt.Errorf("http call: %w", err)
		}

		if resp.StatusCode == http.StatusOK {
			break
		}
		resp.Body.Close()

		if resp.StatusCode != http.StatusAccepted {
			return nil, fmt.Errorf("unexpected status: %s", resp.Status)
		}

		delay += time.Duration(attempt) * time.Second
		if delay > 30*time.Second {
			delay = 30 * time.Second
		}
		select {
		case <-time.After(delay):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	defer resp.Body.Close()

	var raw xmlCollectionItems
	if err = decodeXML(resp.Body, &raw); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	items := make([]CollectionItem, len(raw.Item))
	for i, item := range raw.Item {
		items[i] = CollectionItem{
			ID:            item.ObjectID,
			CollID:        item.CollID,
			Name:          item.Name.Text,
			Type:          ItemType(item.SubType),
			YearPublished: int(safeInt(item.YearPublished)),
			Thumbnail:     item.Thumbnail,
			Image:         item.Image,
			NumPlays:      item.NumPlays,
			Status:        extractCollectionStatus(&item.Status, item.NumPlays),
		}
	}

	return items, nil
}

// --- XML structures (private) ---

type xmlCollectionStatus struct {
	Own              int `xml:"own,attr"`
	PrevOwned        int `xml:"prevowned,attr"`
	ForTrade         int `xml:"fortrade,attr"`
	Want             int `xml:"want,attr"`
	WantToPlay       int `xml:"wanttoplay,attr"`
	WantToTrade      int `xml:"wanttotrade,attr"`
	WantToBuy        int `xml:"wanttobuy,attr"`
	WishList         int `xml:"wishlist,attr"`
	Preordered       int `xml:"preordered,attr"`
	WishListPriority int `xml:"wishlistpriority,attr"`
}

type xmlCollectionItem struct {
	ObjectID      int64               `xml:"objectid,attr"`
	SubType       string              `xml:"subtype,attr"`
	CollID        int64               `xml:"collid,attr"`
	Name          struct{ Text string `xml:",chardata"` } `xml:"name"`
	YearPublished string              `xml:"yearpublished"`
	Image         string              `xml:"image"`
	Thumbnail     string              `xml:"thumbnail"`
	Status        xmlCollectionStatus `xml:"status"`
	NumPlays      int                 `xml:"numplays"`
}

type xmlCollectionItems struct {
	XMLName xml.Name            `xml:"items"`
	Item    []xmlCollectionItem `xml:"item"`
}

func extractCollectionStatus(s *xmlCollectionStatus, numPlays int) []CollectionStatus {
	var out []CollectionStatus
	add := func(cond bool, status CollectionStatus) {
		if cond {
			out = append(out, status)
		}
	}
	add(s.Own != 0, CollectionOwn)
	add(s.Want != 0, CollectionWant)
	add(s.WantToBuy != 0, CollectionWantToBuy)
	add(s.WantToPlay != 0, CollectionWantToPlay)
	add(s.ForTrade != 0 || s.WantToTrade != 0, CollectionTrade)
	add(s.WishList != 0, CollectionWishList)
	add(s.Preordered != 0, CollectionPreorder)
	add(s.PrevOwned != 0, CollectionPrevOwned)
	add(numPlays > 0, CollectionPlayed)
	return out
}
