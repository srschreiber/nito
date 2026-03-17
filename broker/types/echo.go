package types

const EchoMaxChars = 1024

type EchoPayload struct {
	Text string `json:"text"`
}
