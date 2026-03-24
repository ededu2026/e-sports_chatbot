package chat

type Conversation struct {
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	Messages  []Message `json:"messages"`
	UpdatedAt string    `json:"updated_at"`
}
