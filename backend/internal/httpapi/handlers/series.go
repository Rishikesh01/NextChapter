package handlers

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/enable-it/nextchapter/backend/constants"
	"github.com/enable-it/nextchapter/backend/internal/auth"
	"github.com/enable-it/nextchapter/backend/internal/httpapi/render"
	"github.com/enable-it/nextchapter/backend/internal/series"
)

// statusEnumMessage is the canonical 422 message body for an
// out-of-enum status. Built once at init so the per-handler
// validation blocks don't repeat the constant list.
var statusEnumMessage = "must be one of " + strings.Join(constants.AllSeriesStatuses, "|")

// resourceIDUri is the canonical ":id" path-parameter struct shared by
// every handler that takes a numeric resource id in the URL. gin's
// ShouldBindUri populates the int64 and runs the binding tags; a
// failure or non-positive id maps to 404, matching the previous
// hand-rolled behaviour.
type resourceIDUri struct {
	ID int64 `uri:"id" binding:"required,min=1"`
}

// listPaginationQuery is the shared shape for endpoints that accept
// limit / offset query parameters. Embedded into each list endpoint's
// query struct so the defaults and bounds live in one place.
//
// The numbers in the struct tags duplicate
// [constants.ListLimitDefault] / [constants.ListLimitMax] /
// [constants.ListOffsetMin] — Go struct tags can't reference
// constants. Update both when the bounds change.
type listPaginationQuery struct {
	Limit  int `form:"limit,default=50" binding:"omitempty,min=1,max=200"`
	Offset int `form:"offset,default=0" binding:"omitempty,min=0"`
}

// seriesListQuery binds GET /series query parameters.
type seriesListQuery struct {
	Status string `form:"status"`
	listPaginationQuery
}

// SeriesDeps groups the dependencies the series handlers need.
type SeriesDeps struct {
	Series *series.Service
	Logger *zap.Logger
}

// SeriesResponse is the JSON shape for POST/PATCH /series and the base
// shape embedded in summary/detail responses.
type SeriesResponse struct {
	ID        int64     `json:"id"`
	Title     string    `json:"title"`
	Status    string    `json:"status"`
	Rating    *int      `json:"rating"`
	Notes     string    `json:"notes"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// SeriesSummaryResponse is the per-row shape of GET /series — the base
// series fields plus rollup columns.
type SeriesSummaryResponse struct {
	SeriesResponse
	HighestChapter *float64   `json:"highest_chapter"`
	EntryCount     int64      `json:"entry_count"`
	LastCapturedAt *time.Time `json:"last_captured_at"`
}

// SeriesDetailResponse is the GET /series/{id} response: a summary plus
// the full entry list under that series.
type SeriesDetailResponse struct {
	SeriesSummaryResponse
	Entries []EntryResponse `json:"entries"`
}

// SeriesListResponse is the GET /series envelope: paginated summaries
// plus total count for the filtered set.
type SeriesListResponse struct {
	Items []SeriesSummaryResponse `json:"items"`
	Total int64                   `json:"total"`
}

func seriesRowToJSON(r series.Series) SeriesResponse {
	return SeriesResponse{
		ID:        r.ID,
		Title:     r.Title,
		Status:    r.Status,
		Rating:    r.Rating,
		Notes:     r.Notes,
		CreatedAt: r.CreatedAt,
		UpdatedAt: r.UpdatedAt,
	}
}

func summaryToJSON(s series.Summary) SeriesSummaryResponse {
	base := SeriesResponse{
		ID:        s.ID,
		Title:     s.Title,
		Status:    s.Status,
		Rating:    s.Rating,
		Notes:     s.Notes,
		CreatedAt: s.CreatedAt,
		UpdatedAt: s.UpdatedAt,
	}
	return SeriesSummaryResponse{
		SeriesResponse: base,
		HighestChapter: s.HighestChapter,
		EntryCount:     s.EntryCount,
		LastCapturedAt: s.LastCapturedAt,
	}
}

// List implements GET /series.
func (d SeriesDeps) List(c *gin.Context) {
	u, _ := auth.UserFromContext(c.Request.Context())
	var q seriesListQuery
	if err := c.ShouldBindQuery(&q); err != nil {
		render.ValidationError(c, "invalid query", map[string]string{"query": err.Error()})
		return
	}
	status := strings.TrimSpace(q.Status)
	if status != "" && !series.ValidStatus(status) {
		render.ValidationError(c, "invalid status", map[string]string{"status": statusEnumMessage})
		return
	}
	items, total, err := d.Series.List(c.Request.Context(), u.ID, series.ListParams{
		Status: status, Limit: q.Limit, Offset: q.Offset,
	})
	if err != nil {
		if errors.Is(err, series.ErrInvalidStatus) {
			render.ValidationError(c, "invalid status", map[string]string{"status": "invalid"})
			return
		}
		d.Logger.Error("list series", zap.Error(err))
		render.Internal(c, "")
		return
	}
	out := make([]SeriesSummaryResponse, 0, len(items))
	for _, s := range items {
		out = append(out, summaryToJSON(s))
	}
	c.JSON(http.StatusOK, SeriesListResponse{Items: out, Total: total})
}

// Create implements POST /series.
func (d SeriesDeps) Create(c *gin.Context) {
	u, _ := auth.UserFromContext(c.Request.Context())
	var req series.CreateParams
	if !bindJSON(c, &req) {
		return
	}
	req.Title = strings.TrimSpace(req.Title)
	row, err := d.Series.Create(c.Request.Context(), u.ID, req)
	if err != nil {
		d.Logger.Error("create series", zap.Error(err))
		render.Internal(c, "")
		return
	}
	c.JSON(http.StatusCreated, seriesRowToJSON(row))
}

// Get implements GET /series/{id}.
func (d SeriesDeps) Get(c *gin.Context) {
	u, _ := auth.UserFromContext(c.Request.Context())
	var uri resourceIDUri
	if err := c.ShouldBindUri(&uri); err != nil {
		render.NotFound(c, "")
		return
	}
	det, err := d.Series.Detail(c.Request.Context(), u.ID, uri.ID)
	if err != nil {
		if errors.Is(err, series.ErrNotFound) {
			render.NotFound(c, "")
			return
		}
		d.Logger.Error("get series", zap.Error(err))
		render.Internal(c, "")
		return
	}
	entriesJSON := make([]EntryResponse, 0, len(det.Entries))
	for _, e := range det.Entries {
		entriesJSON = append(entriesJSON, entryToJSON(e))
	}
	body := SeriesDetailResponse{
		SeriesSummaryResponse: summaryToJSON(det.Summary),
		Entries:               entriesJSON,
	}
	c.JSON(http.StatusOK, body)
}

// Patch implements PATCH /series/{id}.
func (d SeriesDeps) Patch(c *gin.Context) {
	u, _ := auth.UserFromContext(c.Request.Context())
	var uri resourceIDUri
	if err := c.ShouldBindUri(&uri); err != nil {
		render.NotFound(c, "")
		return
	}
	var req series.UpdateParams
	if !bindJSON(c, &req) {
		return
	}
	if req.Title != nil {
		t := strings.TrimSpace(*req.Title)
		req.Title = &t
	}
	row, err := d.Series.Update(c.Request.Context(), u.ID, uri.ID, req)
	if err != nil {
		if errors.Is(err, series.ErrNotFound) {
			render.NotFound(c, "")
			return
		}
		d.Logger.Error("patch series", zap.Error(err))
		render.Internal(c, "")
		return
	}
	c.JSON(http.StatusOK, seriesRowToJSON(row))
}

// Delete implements DELETE /series/{id}.
func (d SeriesDeps) Delete(c *gin.Context) {
	u, _ := auth.UserFromContext(c.Request.Context())
	var uri resourceIDUri
	if err := c.ShouldBindUri(&uri); err != nil {
		render.NotFound(c, "")
		return
	}
	if err := d.Series.Delete(c.Request.Context(), u.ID, uri.ID); err != nil {
		if errors.Is(err, series.ErrNotFound) {
			render.NotFound(c, "")
			return
		}
		d.Logger.Error("delete series", zap.Error(err))
		render.Internal(c, "")
		return
	}
	c.Status(http.StatusNoContent)
}
