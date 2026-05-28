package contract

type SearchEvent struct {
	Query     string `json:"query"`
	UserID    string `json:"user_id"`
	SessionID string `json:"session_id"`
	RequestID string `json:"request_id"`
	Timestamp int64  `json:"timestamp"`
}
