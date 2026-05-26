package contract

// SearchEvent — событие поискового запроса от смежного сервиса.
// Поля выбраны минимально-достаточными для решения задач:
//   - Query: то, что агрегируем.
//   - UserID: для дедупликации накруток (один пользователь = один голос за окно дедупа).
//   - Timestamp: для отсечения сильно опоздавших событий и корректного бакетирования.
//   - SessionID: опционально, как fallback если UserID нет (анонимы).
//
// Намеренно НЕ включаем: гео, фильтры, категорию — они не нужны для топа.
type SearchEvent struct {
	Query     string `json:"query"`
	UserID    string `json:"user_id"`
	SessionID string `json:"session_id"`
	RequestID string `json:"request_id"`
	Timestamp int64  `json:"timestamp"` // unix seconds
}
