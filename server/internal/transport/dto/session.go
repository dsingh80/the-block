package dto

// SessionInfo is the wire shape of GET /v1/session. Deliberately never
// includes the session token itself: that value already reaches the client
// exactly once, in an httpOnly cookie -- echoing it back in a JSON body would
// hand JavaScript a value the httpOnly flag exists specifically to hide from
// it (guidelines/06-backend-architecture.md, "Sessions & security").
type SessionInfo struct {
	CreatedAt string `json:"created_at"`
}
