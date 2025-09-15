package emailw

import (
	"bytes"
	"crypto/tls"
	"encoding/base64"
	"github.com/pkg/errors"
	"gopkg.in/gomail.v2"
	"html/template"
	"io"
	"strings"
)

type emailW struct {
	dialer *gomail.Dialer
	Sender string
}

func (e emailW) SentEmail(param SentEmailParam) error {
	mailer := gomail.NewMessage()

	if param.Sender != "" {
		mailer.SetHeader("From", param.Sender)
	}

	if len(param.To) > 0 {
		mailer.SetHeader("To", param.To...)
	}

	if len(param.Cc) > 0 {
		for _, cc := range param.Cc {
			mailer.SetAddressHeader("Cc", cc.Email, cc.Name)
		}
	}

	if strings.TrimSpace(param.Subject) != "" {
		mailer.SetHeader("Subject", param.Subject)
	}

	if strings.TrimSpace(param.Message) != "" {
		var messageType = "text/plain"
		if param.MessageType != "" {
			messageType = param.MessageType
		}
		mailer.SetHeader(messageType, param.Message)
	}

	if strings.TrimSpace(param.Template) != "" {
		tmpl, err := template.ParseFiles(param.Template)
		if err != nil {
			return err
		}

		buf := new(bytes.Buffer)
		if err = tmpl.Execute(buf, param.Param); err != nil {
			return errors.Wrap(err, "error execute template")
		}
		mailer.SetBody("text/html", buf.String())
	}

	if len(param.Attachments) > 0 {
		for _, attachment := range param.Attachments {
			decodeString, err := base64.StdEncoding.DecodeString(attachment.FileBase64)
			if err != nil {
				return err
			}
			mailer.Attach(attachment.FileName,
				gomail.SetCopyFunc(func(w io.Writer) error {
					_, err := w.Write(decodeString)
					return err
				}),
				gomail.SetHeader(map[string][]string{
					"Content-Type": {attachment.ContentType},
				}),
			)
		}
	}

	return e.dialer.DialAndSend(mailer)
}

func New(cfg EmailConfig) EmailW {
	dialer := gomail.NewDialer(cfg.Host, cfg.Port, cfg.User, cfg.Password)
	dialer.TLSConfig = &tls.Config{InsecureSkipVerify: true}
	if cfg.UsingSSL {
		dialer.SSL = true
	}
	return emailW{
		dialer: dialer,
	}
}
