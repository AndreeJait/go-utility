package emailw

type EmailW interface {
	SentEmail(param SentEmailParam) error
}
