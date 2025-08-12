package gracefull

import (
	"context"
	"errors"
	"github.com/AndreeJait/go-utility/loggerw"
	"testing"
	"time"
)

func TestGracefulShutDown_ShutdownAll(t *testing.T) {
	type fields struct {
		mapShutDownFunc MapShutdownFunc
		log             loggerw.Logger
	}

	log, _ := loggerw.DefaultLog()
	tests := []struct {
		name   string
		fields fields
	}{
		{
			name: "shutdown success",
			fields: fields{
				log: log,
				mapShutDownFunc: MapShutdownFunc{
					"testing-andree": func() error {
						time.Sleep(2 * time.Second)

						return nil
					},

					"testing-andree-error": func() error {
						time.Sleep(1 * time.Second)
						return errors.New("something error")
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := &GracefulShutDown{
				mapShutDownFunc: tt.fields.mapShutDownFunc,
				log:             tt.fields.log,
			}
			g.ShutdownAll(context.Background())
		})
	}
}
