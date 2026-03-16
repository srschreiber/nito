package types

type PingRequest struct {
	Message string `json:"message" validate:"required,max=256"`
}

type PingResponse struct {
	Message string `json:"message"`
}
