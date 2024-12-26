package model

type Payload struct {
	Message string `json:"message"`
}

type ReplyPayload struct {
	Source        string  `json:"source"`
	CorrelationId string  `json:"correlationId"`
	Duration      string  `json:"duration"`
	Payload       Payload `json:"payload"`
}
