package settlement

import (
	"time"

	"github.com/google/uuid"
)

// Cursor is a position in a keyset-paginated list: the sort timestamp and id
// of the last row already returned. A page continues strictly after it, so
// rows approved or rejected between two requests never shift later pages.
type Cursor struct {
	At time.Time
	ID uuid.UUID
}
