package series

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"image"
	"net/http"
	"time"

	"go.uber.org/zap"

	// Registers the JPEG and PNG decoders with image.DecodeConfig. WebP
	// is not in the standard library and is measured by webpDimensions
	// below rather than by pulling in golang.org/x/image.
	_ "image/jpeg"
	_ "image/png"

	"github.com/enable-it/nextchapter/backend/constants"
	"github.com/enable-it/nextchapter/backend/internal/models"
)

// SetSeriesCover stores (or replaces) the cover image for a series.
//
// The bytes arrive from the extension, which fetched them from the
// page the user was on — the backend never dereferences a URL itself
// (ADR-0011 §1), so this method is the whole ingestion path. It is
// deliberately suspicious of its input: the MIME type is sniffed from
// the bytes rather than taken from the request, the dimensions must
// actually decode, and the size is re-checked even though the handler
// already capped the reader.
func (s *Service) SetSeriesCover(ctx context.Context, userID, seriesID int64, up models.CoverUpload) (models.SeriesCoverMeta, error) {
	// 404 before 422: an unknown series id must not report anything
	// about the payload.
	if _, err := s.repo.getSeriesByID(ctx, userID, seriesID); err != nil {
		return models.SeriesCoverMeta{}, err
	}
	if len(up.Bytes) == 0 {
		return models.SeriesCoverMeta{}, models.ErrCoverEmpty
	}
	if int64(len(up.Bytes)) > constants.MaxCoverBytes {
		return models.SeriesCoverMeta{}, models.ErrCoverUnsupportedType
	}

	mime := http.DetectContentType(up.Bytes)
	switch mime {
	case constants.MimeJPEG, constants.MimePNG, constants.MimeWebP:
	default:
		s.logger.Info("cover rejected: unsupported type",
			zap.Int64("user_id", userID),
			zap.Int64("series_id", seriesID),
			zap.String("sniffed_mime", mime),
		)
		return models.SeriesCoverMeta{}, models.ErrCoverUnsupportedType
	}

	width, height, err := imageDimensions(mime, up.Bytes)
	if err != nil {
		s.logger.Info("cover rejected: undecodable",
			zap.Int64("user_id", userID),
			zap.Int64("series_id", seriesID),
			zap.String("sniffed_mime", mime),
			zap.Error(err),
		)
		return models.SeriesCoverMeta{}, models.ErrCoverUndecodable
	}

	sum := sha256.Sum256(up.Bytes)
	now := time.Now().UTC()
	meta, err := s.repo.upsertSeriesCover(ctx, upsertCoverParams{
		SeriesID:  seriesID,
		UserID:    userID,
		Bytes:     up.Bytes,
		Mime:      mime,
		ByteSize:  int64(len(up.Bytes)),
		Width:     width,
		Height:    height,
		ETag:      hex.EncodeToString(sum[:]),
		SourceURL: up.SourceURL,
		CreatedAt: now,
		UpdatedAt: now,
	})
	if err != nil {
		s.logger.Error("cover: upsert",
			zap.Int64("user_id", userID),
			zap.Int64("series_id", seriesID),
			zap.Error(err),
		)
		return models.SeriesCoverMeta{}, err
	}
	return meta, nil
}

// FindSeriesCover returns the stored cover including its bytes.
// Returns models.ErrCoverNotFound when the series has no cover, which
// is also what an unknown or other-user series id produces — the
// repository query is scoped by user_id, so this never leaks the
// existence of another user's series.
func (s *Service) FindSeriesCover(ctx context.Context, userID, seriesID int64) (models.SeriesCover, error) {
	return s.repo.getSeriesCover(ctx, userID, seriesID)
}

// RemoveSeriesCover deletes the series' cover, returning
// models.ErrCoverNotFound when there was nothing to delete.
func (s *Service) RemoveSeriesCover(ctx context.Context, userID, seriesID int64) error {
	n, err := s.repo.deleteSeriesCover(ctx, userID, seriesID)
	if err != nil {
		s.logger.Error("cover: delete",
			zap.Int64("user_id", userID),
			zap.Int64("series_id", seriesID),
			zap.Error(err),
		)
		return err
	}
	if n == 0 {
		return models.ErrCoverNotFound
	}
	return nil
}

// imageDimensions reads the pixel dimensions out of an image header.
// JPEG and PNG go through image.DecodeConfig (registered by the blank
// imports above); WebP is measured by webpDimensions because the
// standard library has no WebP decoder.
//
// Decoding only the *config* means the full image is never rasterised,
// so a decompression-bomb upload costs header bytes, not pixels.
func imageDimensions(mime string, b []byte) (width, height int64, err error) {
	if mime == constants.MimeWebP {
		return webpDimensions(b)
	}
	cfg, _, err := image.DecodeConfig(bytes.NewReader(b))
	if err != nil {
		return 0, 0, err
	}
	return int64(cfg.Width), int64(cfg.Height), nil
}

// errBadWebP is returned for any malformed WebP header. The caller
// turns every failure into models.ErrCoverUndecodable, so the specific
// shape of the corruption is not worth distinguishing.
type webpError string

func (e webpError) Error() string { return string(e) }

const errBadWebP = webpError("series: malformed WebP header")

// webpDimensions extracts the canvas size from a RIFF/WEBP container.
// The layout is fixed by the WebP spec:
//
//	bytes 0-3   "RIFF"
//	bytes 4-7   file size (ignored — we already hold the whole buffer)
//	bytes 8-11  "WEBP"
//	bytes 12-15 chunk FourCC: "VP8 " (lossy), "VP8L" (lossless) or "VP8X" (extended)
//	bytes 16-19 chunk payload size
//	bytes 20..  payload
//
// Each of the three chunk kinds stores the dimensions differently, and
// all three pack them into 14- or 24-bit little-endian fields.
func webpDimensions(b []byte) (width, height int64, err error) {
	if len(b) < 30 || !bytes.Equal(b[0:4], []byte("RIFF")) || !bytes.Equal(b[8:12], []byte("WEBP")) {
		return 0, 0, errBadWebP
	}
	payload := b[20:]

	switch string(b[12:16]) {
	case "VP8 ":
		// Lossy: 3-byte frame tag, then the 3-byte sync code
		// 0x9d 0x01 0x2a, then two 14-bit dimensions.
		if len(payload) < 10 || !bytes.Equal(payload[3:6], []byte{0x9d, 0x01, 0x2a}) {
			return 0, 0, errBadWebP
		}
		w := int64(binary.LittleEndian.Uint16(payload[6:8]) & 0x3FFF)
		h := int64(binary.LittleEndian.Uint16(payload[8:10]) & 0x3FFF)
		return nonZeroDims(w, h)

	case "VP8L":
		// Lossless: 0x2f signature byte, then 14 bits of (width-1)
		// and 14 bits of (height-1) packed into the next 4 bytes.
		if len(payload) < 5 || payload[0] != 0x2f {
			return 0, 0, errBadWebP
		}
		bits := binary.LittleEndian.Uint32(payload[1:5])
		w := int64(bits&0x3FFF) + 1
		h := int64((bits>>14)&0x3FFF) + 1
		return nonZeroDims(w, h)

	case "VP8X":
		// Extended: 4 flag bytes, then 24-bit (canvas width-1) and
		// 24-bit (canvas height-1), both little-endian.
		if len(payload) < 10 {
			return 0, 0, errBadWebP
		}
		w := int64(uint32(payload[4])|uint32(payload[5])<<8|uint32(payload[6])<<16) + 1
		h := int64(uint32(payload[7])|uint32(payload[8])<<8|uint32(payload[9])<<16) + 1
		return nonZeroDims(w, h)

	default:
		return 0, 0, errBadWebP
	}
}

// nonZeroDims guards the CHECK constraints on series_cover.width /
// height, which reject non-positive values at the database layer.
func nonZeroDims(w, h int64) (int64, int64, error) {
	if w <= 0 || h <= 0 {
		return 0, 0, errBadWebP
	}
	return w, h, nil
}
