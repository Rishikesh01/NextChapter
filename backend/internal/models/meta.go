package models

// Health is the GET /healthz wire shape. It has no service-level
// domain backing — the meta handler builds it directly from the
// pre-wired version string — but it lives here so handlers do not
// invent parallel response types alongside the [internal/models]
// package.
type Health struct {
	Status  string `json:"status"`
	Version string `json:"version,omitempty"`
}
