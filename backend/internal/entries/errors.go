package entries

import "github.com/enable-it/nextchapter/backend/internal/models"

// Re-exported sentinels so callers inside this package keep the
// short name. The canonical values live in [models] so handlers can
// errors.Is without importing this package.
var (
	ErrNotFound       = models.ErrEntryNotFound
	ErrSeriesRequired = models.ErrEntryCaptureSeriesRequired
	ErrSeriesNotFound = models.ErrSeriesNotFound
)
