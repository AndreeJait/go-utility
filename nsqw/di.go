package nsqw

import "context"

type NsqW interface {
	Start() error
	AddHandler(param AddHandlerParam) error
	Disconnect(ctx context.Context)
}
