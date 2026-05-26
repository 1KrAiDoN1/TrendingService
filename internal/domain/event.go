package domain

import "time"

// SearchEvent представляет событие поискового запроса.
// Это доменная сущность, которая используется внутри сервиса.
type SearchEvent struct {
	Query     string
	UserID    string
	SessionID string
	RequestID string
	Timestamp time.Time
}

// IsValid проверяет валидность события.
func (e *SearchEvent) IsValid() bool {
	return e.Query != "" && (e.UserID != "" || e.SessionID != "")
}

// GetDeduplicationKey возвращает ключ для дедупликации.
// Приоритет отдается UserID, если он есть.
func (e *SearchEvent) GetDeduplicationKey() string {
	if e.UserID != "" {
		return e.UserID
	}
	return e.SessionID
}
