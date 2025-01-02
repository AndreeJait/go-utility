package nsqw

import (
	"context"
	"github.com/nsqio/go-nsq"
)

type AddHandlerParam struct {
	Topic       string `json:"topic"`
	Channel     string `json:"channel"`
	MaxAttempts uint16 `json:"max_attempts"`

	Handler func(ctx context.Context, message *nsq.Message) error

	MaxInFlight           int `json:"max_in_flight"`
	MaxRequeueInDelay     int `json:"max_requeue_in_delay"`
	DefaultRequeueInDelay int `json:"default_requeue_in_delay"`
}

type Config struct {
	Hosts   []Host `json:"hosts"`
	IsDebug bool   `json:"is_debug"`
}

type Host struct {
	Host string `json:"host"`
	Port string `json:"port"`
}
