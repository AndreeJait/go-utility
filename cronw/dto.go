package cronw

import "context"

type AddHandlerParam struct {
	Pattern string `json:"pattern"`
	Handler func(ctx context.Context) error
	Name    string `json:"name"`
}
