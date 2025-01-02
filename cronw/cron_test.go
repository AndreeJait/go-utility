package cronw

import (
	"context"
	"github.com/AndreeJait/go-utility/loggerw"
	"testing"
	"time"
)

func TestCronW(t *testing.T) {
	t.Run("testing run cron", func(t *testing.T) {
		location, _ := time.LoadLocation("Asia/Jakarta")
		log, _ := loggerw.DefaultLog()
		scheduler := New(location, log)

		scheduler.AddHandler(AddHandlerParam{
			Handler: func(ctx context.Context) error {
				log.Infof("cron is executed at %v", time.Now())
				return nil
			},
			Pattern: "*/1 * * * *",
			Name:    "andree-testing",
		})

		scheduler.Start()
	})
}
