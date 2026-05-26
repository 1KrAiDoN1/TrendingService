package domain

// TopEntry представляет элемент топа популярных запросов.
type TopEntry struct {
	Query string `json:"query"`
	Count int64  `json:"count"`
}

// TopSnapshot представляет снимок топа запросов в определенный момент времени.
type TopSnapshot struct {
	Entries     []TopEntry `json:"entries"`
	GeneratedAt int64      `json:"generated_at"` // unix nano
	WindowSec   int        `json:"window_sec"`
}
