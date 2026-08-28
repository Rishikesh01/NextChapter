package constants

// MaxCoverBytes caps a series cover upload at 5MiB. The handler
// enforces it with http.MaxBytesReader so a hostile client cannot
// exhaust memory before the service ever sees the body, and the
// service re-checks it so a non-HTTP caller cannot bypass the limit
// (ADR-0011 §4). Real covers run 50-200KB; 5MiB is generous headroom
// for a high-resolution scan without inviting abuse.
const MaxCoverBytes = 5 << 20

// Cover MIME types accepted by the upload endpoint. The type is
// sniffed from the uploaded bytes via http.DetectContentType — the
// request's declared Content-Type is never trusted. Mirrors the CHECK
// constraint on series_cover.mime; keep the two in sync.
const (
	MimeJPEG = "image/jpeg"
	MimePNG  = "image/png"
	MimeWebP = "image/webp"
)
