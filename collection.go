package bggo

import (
	"context"
	"encoding/xml"
	"fmt"
	"net/http"
	"strconv"
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
	// Stats includes the per-item <stats> block in the response,
	// which carries the requesting user's personal rating value
	// among other fields. Maps to ?stats=1.
	Stats bool
	// ShowPrivate includes the <privateinfo> block (pricepaid,
	// pp_currency, acquisitiondate, acquiredfrom,
	// inventorylocation, <privatecomment>) in the response. Only
	// honored by BGG when the request's session cookie belongs to
	// the user named in Username — anonymous calls and other-user
	// calls receive the public projection regardless. Maps to
	// ?showprivate=1.
	ShowPrivate bool
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
	if r.Stats {
		p["stats"] = "1"
	}
	if r.ShowPrivate {
		p["showprivate"] = "1"
	}

	return p
}

// CollectionPrivateInfo carries the per-item privateinfo block
// from a /collection?showprivate=1 fetch. Only populated when the
// request was made with the owning user's session cookie AND
// GetCollectionRequest.ShowPrivate was set; otherwise nil.
type CollectionPrivateInfo struct {
	PricePaid         string `json:"price_paid,omitempty"`
	PriceCurrency     string `json:"price_currency,omitempty"`
	AcquisitionDate   string `json:"acquisition_date,omitempty"`
	AcquiredFrom      string `json:"acquired_from,omitempty"`
	InventoryLocation string `json:"inventory_location,omitempty"`
	PrivateComment    string `json:"private_comment,omitempty"`
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
	// Rating is the requesting user's personal rating for the
	// item, populated only when GetCollectionRequest.Stats is set.
	// Nil when the item is unrated (BGG renders unrated items as
	// rating value="N/A"); a non-nil pointer to a value in [1, 10]
	// (half-points allowed) otherwise.
	Rating *float64 `json:"rating,omitempty"`
	// Comment is the requesting user's public comment on the item.
	// Empty when unset.
	Comment string `json:"comment,omitempty"`
	// PrivateInfo carries the privateinfo block when
	// GetCollectionRequest.ShowPrivate was set AND the request's
	// session cookie belongs to the user named in Username.
	// Nil otherwise.
	PrivateInfo *CollectionPrivateInfo `json:"private_info,omitempty"`

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
			return nil, newHTTPStatusError(resp)
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
			Rating:        extractCollectionRating(item.Stats),
			Comment:       item.Comment,
			PrivateInfo:   extractCollectionPrivateInfo(item.PrivateInfo),
		}
	}

	return items, nil
}

// extractCollectionRating decodes the <stats><rating value="..."/>
// element. BGG renders unrated items as value="N/A"; a non-nil
// pointer is returned only for a numeric value parseable as a
// float64 (half-point ratings like 8.5 are legal upstream).
func extractCollectionRating(s *xmlCollectionStats) *float64 {
	if s == nil {
		return nil
	}
	raw := strings.TrimSpace(s.Rating.Value)
	if raw == "" || strings.EqualFold(raw, "N/A") {
		return nil
	}
	v, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return nil
	}
	return &v
}

// extractCollectionPrivateInfo maps the privateinfo XML block to
// the public struct. Returns nil when the wire element is absent
// AND all attrs/children are empty — distinguishes "no privateinfo
// shown" (anonymous / other-user call) from "privateinfo shown but
// every field is the zero value" (unlikely but expressible).
func extractCollectionPrivateInfo(p *xmlCollectionPrivateInf) *CollectionPrivateInfo {
	if p == nil {
		return nil
	}
	out := &CollectionPrivateInfo{
		PricePaid:         p.PricePaid,
		PriceCurrency:     p.PriceCurrency,
		AcquisitionDate:   p.AcquisitionDate,
		AcquiredFrom:      p.AcquiredFrom,
		InventoryLocation: p.InventoryLocation,
		PrivateComment:    p.PrivateComment,
	}
	if out.PricePaid == "" && out.PriceCurrency == "" &&
		out.AcquisitionDate == "" && out.AcquiredFrom == "" &&
		out.InventoryLocation == "" && out.PrivateComment == "" {
		return nil
	}
	return out
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
	ObjectID int64  `xml:"objectid,attr"`
	SubType  string `xml:"subtype,attr"`
	CollID   int64  `xml:"collid,attr"`
	Name     struct {
		Text string `xml:",chardata"`
	} `xml:"name"`
	YearPublished string                   `xml:"yearpublished"`
	Image         string                   `xml:"image"`
	Thumbnail     string                   `xml:"thumbnail"`
	Status        xmlCollectionStatus      `xml:"status"`
	NumPlays      int                      `xml:"numplays"`
	Comment       string                   `xml:"comment"`
	Stats         *xmlCollectionStats      `xml:"stats"`
	PrivateInfo   *xmlCollectionPrivateInf `xml:"privateinfo"`
}

type xmlCollectionStats struct {
	Rating struct {
		Value string `xml:"value,attr"`
	} `xml:"rating"`
}

type xmlCollectionPrivateInf struct {
	PricePaid         string `xml:"pricepaid,attr"`
	PriceCurrency     string `xml:"pp_currency,attr"`
	AcquisitionDate   string `xml:"acquisitiondate,attr"`
	AcquiredFrom      string `xml:"acquiredfrom,attr"`
	InventoryLocation string `xml:"inventorylocation,attr"`
	PrivateComment    string `xml:"privatecomment"`
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
