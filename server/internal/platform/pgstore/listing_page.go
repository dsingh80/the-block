package pgstore

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/dsingh80/the-block/server/internal/domain"
	"github.com/dsingh80/the-block/server/internal/usecase/listings"
)

// simpleSortColumn describes the four sort modes' single-predicate ORDER BY.
// "ending" isn't here -- it's bucketed (endingBuckets below), not a single
// column (guidelines/06-backend-architecture.md, "Cursor pagination").
var simpleSortColumn = map[listings.SortMode]struct {
	column    string
	ascending bool
}{
	listings.SortPriceLow:  {"current_price", true},
	listings.SortPriceHigh: {"current_price", false},
	listings.SortYear:      {"year", false},
}

// endingBucket is one of the 3 buckets sort=ending walks through, in forward
// order: active (soonest-ending first), upcoming (soonest-starting first),
// ended (a new, explicit tiebreak -- the client's original comparator never
// actually ordered ended listings among themselves).
type endingBucket struct {
	index     int
	predicate string
	column    string
	ascending bool
}

var endingBuckets = []endingBucket{
	{0, "auction_start <= now() AND auction_end > now() AND purchased_at IS NULL", "auction_end", true},
	{1, "auction_start > now()", "auction_start", true},
	{2, "purchased_at IS NOT NULL OR auction_end <= now()", "auction_end", false},
}

func bucketForListing(l domain.Listing, now time.Time) int {
	switch l.Status(now) {
	case domain.LifecycleActive:
		return 0
	case domain.LifecycleUpcoming:
		return 1
	default:
		return 2
	}
}

// ListPage is the cursor-paginated list query (guidelines/06-backend-architecture.md).
func (r *ListingReader) ListPage(ctx context.Context, req listings.PageRequest) (listings.Page, error) {
	forward := req.Before == ""
	limit := req.First
	cursorStr := req.After
	if !forward {
		limit = req.Last
		cursorStr = req.Before
	}
	if limit <= 0 {
		limit = 24
	}

	var cur *listings.Cursor
	if cursorStr != "" {
		decoded, err := listings.DecodeCursor(cursorStr)
		if err != nil {
			return listings.Page{}, err
		}
		if decoded.Sort != req.Sort || decoded.Fingerprint != listings.Fingerprint(req.Sort, req.Filter) {
			return listings.Page{}, domain.ErrCursorSortMismatch
		}
		cur = &decoded
	}

	var rows []domain.Listing
	var err error
	if req.Sort == listings.SortEnding {
		rows, err = r.listEndingPage(ctx, req.Filter, cur, forward, limit+1)
	} else {
		spec := simpleSortColumn[req.Sort]
		rows, err = r.listSimplePage(ctx, req.Filter, "", spec.column, spec.ascending, cur, forward, limit+1)
	}
	if err != nil {
		return listings.Page{}, err
	}

	hasMore := len(rows) > limit
	if hasMore {
		rows = rows[:limit]
	}
	if !forward {
		reverseListings(rows)
	}

	page := listings.Page{Items: rows}
	if forward {
		page.HasNextPage = hasMore
		page.HasPrevPage = cur != nil
	} else {
		page.HasPrevPage = hasMore
		page.HasNextPage = cur != nil
	}

	if len(rows) > 0 {
		now := time.Now()
		page.StartCursor = listings.EncodeCursor(cursorFor(rows[0], req.Sort, req.Filter, now))
		page.EndCursor = listings.EncodeCursor(cursorFor(rows[len(rows)-1], req.Sort, req.Filter, now))
	}
	return page, nil
}

// listEndingPage walks the 3 buckets in order, only resuming from the cursor
// on the first bucket it touches -- every subsequent bucket starts fresh at
// its own beginning (guidelines/06-backend-architecture.md).
func (r *ListingReader) listEndingPage(ctx context.Context, filter listings.Filter, cur *listings.Cursor, forward bool, limit int) ([]domain.Listing, error) {
	order := []endingBucket{endingBuckets[0], endingBuckets[1], endingBuckets[2]}
	if !forward {
		order = []endingBucket{endingBuckets[2], endingBuckets[1], endingBuckets[0]}
	}

	startPos := 0
	if cur != nil {
		for i, b := range order {
			if b.index == cur.Bucket {
				startPos = i
				break
			}
		}
	}

	var result []domain.Listing
	activeCursor := cur
	for pos := startPos; pos < len(order) && len(result) < limit; pos++ {
		b := order[pos]
		rows, err := r.listSimplePage(ctx, filter, b.predicate, b.column, b.ascending, activeCursor, forward, limit-len(result))
		if err != nil {
			return nil, err
		}
		result = append(result, rows...)
		activeCursor = nil
	}
	return result, nil
}

// listSimplePage is the single-predicate keyset query shared by every sort
// mode: the 3 non-bucketed ones directly, and each of sort=ending's 3 buckets
// (with their own extraPredicate) via listEndingPage.
func (r *ListingReader) listSimplePage(ctx context.Context, filter listings.Filter, extraPredicate, column string, ascending bool, cur *listings.Cursor, forward bool, limit int) ([]domain.Listing, error) {
	naturalOrder, cmp := "ASC", ">"
	if !ascending {
		naturalOrder, cmp = "DESC", "<"
	}
	queryOrder := naturalOrder
	if !forward {
		if queryOrder == "ASC" {
			queryOrder, cmp = "DESC", "<"
		} else {
			queryOrder, cmp = "ASC", ">"
		}
	}

	where, args := filterWhere(filter, 1)
	if extraPredicate != "" {
		where = append(where, extraPredicate)
	}

	if cur != nil {
		val, err := parseColumnValue(column, cur.Value)
		if err != nil {
			return nil, domain.ErrInvalidCursor
		}
		where = append(where, fmt.Sprintf("(%s, id) %s ($%d, $%d)", column, cmp, len(args)+1, len(args)+2))
		args = append(args, val, cur.ID)
	}

	query := fmt.Sprintf(`SELECT %s FROM listings WHERE %s ORDER BY %s %s, id %s LIMIT $%d`,
		listingColumns, strings.Join(where, " AND "), column, queryOrder, queryOrder, len(args)+1)
	args = append(args, limit)

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("pgstore: list page query: %w", err)
	}
	defer rows.Close()

	var result []domain.Listing
	for rows.Next() {
		l, err := scanListing(rows)
		if err != nil {
			return nil, fmt.Errorf("pgstore: scan listing: %w", err)
		}
		result = append(result, l)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("pgstore: iterate page: %w", err)
	}
	return result, nil
}

func parseColumnValue(column, value string) (any, error) {
	switch column {
	case "current_price":
		return strconv.ParseInt(value, 10, 64)
	case "year":
		return strconv.Atoi(value)
	case "auction_end", "auction_start":
		// RFC3339Nano, not RFC3339: Postgres timestamps carry microsecond precision, and
		// truncating a cursor's encoded value to whole seconds (RFC3339) let it fall
		// strictly before the very row it was issued for, so a ">" keyset predicate
		// against that truncated value re-matched that same row on the next page
		// (guidelines/06-backend-architecture.md, "Cursor pagination").
		return time.Parse(time.RFC3339Nano, value)
	default:
		return nil, fmt.Errorf("pgstore: unrecognized cursor column %q", column)
	}
}

// cursorFor builds the cursor a client would use to resume immediately after
// (or before) this row. now is passed in rather than re-read per row so every
// row in one page is bucketed against the same instant.
func cursorFor(l domain.Listing, sort listings.SortMode, filter listings.Filter, now time.Time) listings.Cursor {
	c := listings.Cursor{Sort: sort, Fingerprint: listings.Fingerprint(sort, filter), ID: l.ID}
	switch sort {
	case listings.SortPriceLow, listings.SortPriceHigh:
		c.Value = strconv.FormatInt(l.CurrentPrice, 10)
	case listings.SortYear:
		c.Value = strconv.Itoa(l.Year)
	case listings.SortEnding:
		bucket := bucketForListing(l, now)
		c.Bucket = bucket
		if bucket == 1 {
			c.Value = l.AuctionStart.UTC().Format(time.RFC3339Nano)
		} else {
			c.Value = l.AuctionEnd.UTC().Format(time.RFC3339Nano)
		}
	}
	return c
}

func reverseListings(l []domain.Listing) {
	for i, j := 0, len(l)-1; i < j; i, j = i+1, j-1 {
		l[i], l[j] = l[j], l[i]
	}
}
