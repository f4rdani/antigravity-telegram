package engine

type StreamUserMessage struct {
	Content string `json:"content"`
}

type StreamInputMessage struct {
	Event   string             `json:"event"` // "user"
	Message *StreamUserMessage `json:"message,omitempty"`
}

type TokenUsage struct {
	InputTokens     int `json:"input_tokens"`
	OutputTokens    int `json:"output_tokens"`
	ThinkingTokens  int `json:"thinking_tokens"`
	CacheReadTokens int `json:"cache_read_tokens"`
	TotalTokens     int `json:"total_tokens"`
}

type StreamInitPayload struct {
	CWD            string   `json:"cwd"`
	Tools          []string `json:"tools"`
	PermissionMode string   `json:"permission_mode"`
	Model          string   `json:"model,omitempty"`
}

type ToolInfoPayload struct {
	Name       string                 `json:"name,omitempty"`
	Parameters map[string]interface{} `json:"parameters,omitempty"`
	Output     string                 `json:"output,omitempty"`
}

type StepUpdatePayload struct {
	ConversationID  string           `json:"conversation_id"`
	StepIndex       int              `json:"step_index"`
	State           string           `json:"state"`     // "ACTIVE", "DONE"
	StepType        string           `json:"step_type"` // "user_input", "agent_response", "system_message", "tool"
	ToolName        string           `json:"tool_name,omitempty"`
	ToolInfo        *ToolInfoPayload `json:"tool_info,omitempty"`
	TextDelta       string           `json:"text_delta,omitempty"`
	DurationSeconds float64          `json:"duration_seconds,omitempty"`
	Usage           *TokenUsage      `json:"usage,omitempty"`
}

type ResultPayload struct {
	ConversationID  string      `json:"conversation_id"`
	Status          string      `json:"status"` // "SUCCESS", "ERROR"
	Response        string      `json:"response"`
	Error           string      `json:"error,omitempty"`
	DurationSeconds float64     `json:"duration_seconds"`
	NumTurns        int         `json:"num_turns"`
	Usage           *TokenUsage `json:"usage,omitempty"`
}

type StreamEvent struct {
	Event          string             `json:"event"`
	ConversationID string             `json:"conversation_id,omitempty"`
	Init           *StreamInitPayload `json:"init,omitempty"`
	StepUpdate     *StepUpdatePayload `json:"step_update,omitempty"`
	Result         *ResultPayload     `json:"result,omitempty"`
}
