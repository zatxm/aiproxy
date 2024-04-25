package types

type BingCompletionRequest struct {
	Conversation *BingConversation `json:"conversation,omitempty"`
	ImageBase64  string            `json:"imageBase64"`
}

type BingConversation struct {
	ConversationId string `json:"conversationId"`
	ClientId       string `json:"clientId"`
	Signature      string `json:"signature"`
	TraceId        string `json:"traceId"`
	ImageUrl       string `json:"imageUrl,omitempty"`
}
