package cronw

type CronW interface {
	Start()
	AddHandler(param AddHandlerParam)
	Stop()
}
