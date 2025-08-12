package gracefull

import (
	"context"
	"github.com/AndreeJait/go-utility/loggerw"
	"sync"
)

type GracefulShutDown struct {
	mapShutDownFunc MapShutdownFunc
	log             loggerw.Logger
}

func NewGracefulShutdown(log loggerw.Logger) *GracefulShutDown {
	return &GracefulShutDown{
		mapShutDownFunc: make(MapShutdownFunc),
		log:             log,
	}
}

func (g *GracefulShutDown) AddFunc(key string, callbackFunc func() error) {
	g.mapShutDownFunc[key] = callbackFunc
}

func (g *GracefulShutDown) ShutdownAll(ctx context.Context) {
	var wg sync.WaitGroup
	for key, val := range g.mapShutDownFunc {
		wg.Add(1)
		go func(key string, callbackFunc func() error) {
			defer wg.Done()
			g.log.Infof(ctx, "starting to shutdown %s", key)

			err := callbackFunc()
			if err != nil {
				g.log.Errorf(ctx, err, "failed to shutdown [%s], reason %+v", key, err)
				return
			}
			g.log.Infof(ctx, "success to shutdown %s", key)
		}(key, val)
	}
	wg.Wait()
	g.log.Infof(ctx, "all process done :)")
}
