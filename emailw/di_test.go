package emailw

import (
	"fmt"
	"testing"
)

func TestSentEmail(t *testing.T) {
	t.Run("sent email", func(t *testing.T) {
		mailer := New(EmailConfig{
			Host:     "sandbox.smtp.mailtrap.io",
			Port:     587,
			User:     "user",
			Password: "password",
		})

		err := mailer.SentEmail(SentEmailParam{
			Subject: "Testing Andree here",
			Sender:  "panjaitanandree@gmail.com",
			To:      []string{"panjaitan@gmail.com", "andree@gmail.com"},
			Cc: []SentEmailParamCC{
				{
					Email: "testing1@gmail.com",
					Name:  "testing 1",
				},
				{
					Email: "testing2@gmail.com",
					Name:  "testing 2",
				},
			},
			Message: "Teting Hi Andree",
		})
		if err != nil {
			fmt.Println(err)
		}
	})

	t.Run("sent email", func(t *testing.T) {
		mailer := New(EmailConfig{
			Host:     "sandbox.smtp.mailtrap.io",
			Port:     587,
			User:     "user",
			Password: "password",
		})

		err := mailer.SentEmail(SentEmailParam{
			Subject: "Testing Andree here",
			Sender:  "panjaitanandree@gmail.com",
			To:      []string{"panjaitan@gmail.com", "andree@gmail.com"},
			Cc: []SentEmailParamCC{
				{
					Email: "testing1@gmail.com",
					Name:  "testing 1",
				},
				{
					Email: "testing2@gmail.com",
					Name:  "testing 2",
				},
			},
			Template: "./example.html",
			Param: map[string]interface{}{
				"name": "Andree Panjaitan",
			},
			Attachments: []SentEmailParamAttach{
				{
					FileName:    "exampletest.txt",
					ContentType: "text/plain",
					FileBase64:  "SGVsbG8gQW5kcmVl",
				},
			},
		})
		if err != nil {
			fmt.Println(err)
		}
	})
}
