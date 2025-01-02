package nsqw

import (
	"context"
	"fmt"
	"github.com/AndreeJait/go-utility/loggerw"
	"github.com/nsqio/go-nsq"
	"os"
	"os/signal"
	"syscall"
	"time"
)

type nsqw struct {
	cfg       Config
	consumers []*nsq.Consumer
	log       loggerw.Logger
}

func (n *nsqw) Start() error {
	var addresses []string

	for _, host := range n.cfg.Hosts {
		address := fmt.Sprintf("%s:%s", host.Host, host.Port)
		addresses = append(addresses, address)
		n.log.Infof("address %s added\n", address)
	}

	for _, consumer := range n.consumers {
		if err := consumer.ConnectToNSQLookupds(addresses); err != nil {
			return err
		}
	}

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan
	n.Disconnect(context.Background())
	n.log.Infof("all instance is stopped\n")
	return nil
}

func (n *nsqw) AddHandler(param AddHandlerParam) error {
	config := nsq.NewConfig()

	config.MaxAttempts = param.MaxAttempts
	config.MaxInFlight = param.MaxInFlight
	config.MaxRequeueDelay = time.Duration(param.MaxRequeueInDelay) * time.Second
	config.DefaultRequeueDelay = time.Duration(param.DefaultRequeueInDelay) * time.Second

	consumer, err := nsq.NewConsumer(param.Topic, param.Channel, config)
	if err != nil {
		return err
	}

	consumer.AddHandler(&messageHandler{
		log:     n.log,
		Handler: param.Handler,
	})
	n.consumers = append(n.consumers, consumer)
	return nil
}

func (n *nsqw) Disconnect(ctx context.Context) {
	for _, consumer := range n.consumers {
		consumer.Stop()
	}
}

type messageHandler struct {
	log     loggerw.Logger
	Handler func(ctx context.Context, message *nsq.Message) error
}

func (m messageHandler) HandleMessage(message *nsq.Message) error {
	return m.Handler(context.Background(), message)
}

func New(cfg Config, log loggerw.Logger) NsqW {
	return &nsqw{
		cfg: cfg,
		log: log,
	}
}
