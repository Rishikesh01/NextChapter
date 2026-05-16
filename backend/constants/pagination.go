package constants

// Pagination defaults and bounds for list endpoints (/series, /entries
// today; future list endpoints reuse the same numbers).
//
// These values are duplicated as struct-tag literals in the handler
// query DTOs because Go struct tags can't reference constants. When
// changing them, update both places — see the comment on
// `seriesListQuery` / `entriesListQuery`.
const (
	// ListLimitDefault is applied when the caller omits ?limit=.
	ListLimitDefault = 50
	// ListLimitMax caps ?limit= to avoid runaway scans.
	ListLimitMax = 200
	// ListOffsetMin is the lower bound on ?offset=; negative offsets are
	// clamped here.
	ListOffsetMin = 0
)
