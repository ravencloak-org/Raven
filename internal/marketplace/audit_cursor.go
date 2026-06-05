// Opaque cursor encoding for the takedown audit-log pagination.
//
// The encoding is `base64url(unix_micros + ":" + uuid)`. We chose
// base64url + a simple delimited body (vs. JSON) because:
//
//   - The cursor is round-tripped through URLs (?cursor=...); base64url
//     keeps it URL-safe without percent-escaping.
//   - The shape is fixed (always two fields); JSON adds bytes and a
//     parser dependency without enabling anything we'd actually use.
//   - "Opaque" is enforced at the API boundary — clients are not allowed
//     to construct cursors themselves, and a parse error returns 400
//     rather than leaking the format.
//
// Microsecond precision matches Postgres's TIMESTAMPTZ resolution (it
// stores microseconds internally). Going finer would lose information
// on round-trip; going coarser would split rows from the same insert
// across pages.

package marketplace

import (
	"encoding/base64"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
)

// ErrInvalidAuditCursor is returned by parseAuditCursor when the supplied
// cursor cannot be decoded. Surfaced through the handler as HTTP 400.
var ErrInvalidAuditCursor = errors.New("marketplace: invalid audit cursor")

// EncodeAuditCursor builds the opaque cursor pointing at (createdAt, id).
// Callers should treat the returned string as opaque; the format is an
// implementation detail.
func EncodeAuditCursor(createdAt time.Time, id uuid.UUID) string {
	// Unix microseconds keeps the encoded string short and round-trip
	// stable: a Go time.Time can be reconstructed exactly from a micros
	// counter (Postgres-precision aligned, see file doc).
	body := strconv.FormatInt(createdAt.UnixMicro(), 10) + ":" + id.String()
	return base64.RawURLEncoding.EncodeToString([]byte(body))
}

// parseAuditCursor decodes a cursor produced by EncodeAuditCursor. Any
// failure (bad base64, bad delimiter, bad timestamp, bad uuid) maps to
// ErrInvalidAuditCursor — callers don't get a finer-grained diagnostic
// because the cursor is opaque by contract.
func parseAuditCursor(cursor string) (time.Time, uuid.UUID, error) {
	raw, err := base64.RawURLEncoding.DecodeString(cursor)
	if err != nil {
		return time.Time{}, uuid.Nil, fmt.Errorf("%w: base64: %v", ErrInvalidAuditCursor, err)
	}
	parts := strings.SplitN(string(raw), ":", 2)
	if len(parts) != 2 {
		return time.Time{}, uuid.Nil, ErrInvalidAuditCursor
	}
	micros, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return time.Time{}, uuid.Nil, fmt.Errorf("%w: micros: %v", ErrInvalidAuditCursor, err)
	}
	id, err := uuid.Parse(parts[1])
	if err != nil {
		return time.Time{}, uuid.Nil, fmt.Errorf("%w: uuid: %v", ErrInvalidAuditCursor, err)
	}
	return time.UnixMicro(micros).UTC(), id, nil
}
