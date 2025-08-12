package cronw

import (
	"context"
	"github.com/AndreeJait/go-utility/loggerw"
	cron "github.com/robfig/cron/v3"
	"os"
	"os/signal"
	"syscall"
	"time"
)

type cronW struct {
	scheduler *cron.Cron
	ids       []cron.EntryID
	log       loggerw.Logger
}

func (c *cronW) Start() {
	var ctx = context.Background()
	c.log.Infof(ctx, "cron is started")
	go c.scheduler.Start()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	c.Stop()
	c.log.Infof(ctx, "cron is stopped")
}

func (c *cronW) AddHandler(param AddHandlerParam) {
	ctx := context.Background()
	ids, err := c.scheduler.AddFunc(param.Pattern, func() {
		errInternal := param.Handler(context.Background())
		if errInternal != nil {
			c.log.Fatal(ctx, errInternal)
		}
	})
	if err != nil {
		c.log.Fatal(ctx, err)
	}

	c.log.Infof(ctx, "cron %s is added", param.Name)
	c.ids = append(c.ids, ids)
}

func (c *cronW) Stop() {
	c.scheduler.Stop()
}

func New(location *time.Location, log loggerw.Logger) CronW {
	scheduler := cron.New(cron.WithLocation(location))
	return &cronW{
		scheduler: scheduler,
		log:       log,
	}
}
