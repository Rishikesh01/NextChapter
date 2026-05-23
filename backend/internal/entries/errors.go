package entries

import "github.com/enable-it/nextchapter/backend/internal/models"

// Re-exported sentinels so callers inside this package keep the
// short name. The canonical values live in [models] so handlers can
// errors.Is without importing this package.
var (
	errNotFound       = models.ErrEntryNotFound
	errSeriesRequired = models.ErrEntryCaptureSeriesRequired
	errSeriesNotFound = models.ErrSeriesNotFound
)
