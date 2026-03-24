package chat

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type Request struct {
	Message        string    `json:"message"`
	History        []Message `json:"history"`
	ConversationID string    `json:"conversation_id,omitempty"`
}

type Response struct {
	Answer         string `json:"answer"`
	Language       string `json:"language"`
	Reason         string `json:"reason,omitempty"`
	Blocked        bool   `json:"blocked"`
	Model          string `json:"model,omitempty"`
	DomainAllowed  bool   `json:"domain_allowed"`
	ConversationID string `json:"conversation_id,omitempty"`
	Cached         bool   `json:"cached,omitempty"`
}

type StreamEvent struct {
	Type           string `json:"type"`
	Content        string `json:"content,omitempty"`
	Answer         string `json:"answer,omitempty"`
	Language       string `json:"language,omitempty"`
	Reason         string `json:"reason,omitempty"`
	Blocked        bool   `json:"blocked,omitempty"`
	Model          string `json:"model,omitempty"`
	DomainAllowed  bool   `json:"domain_allowed,omitempty"`
	ConversationID string `json:"conversation_id,omitempty"`
	Cached         bool   `json:"cached,omitempty"`
	Error          string `json:"error,omitempty"`
}
