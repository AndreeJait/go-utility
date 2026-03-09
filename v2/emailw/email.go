package emailw

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"html/template"
	"mime"
	"net/smtp"
	"path/filepath"
	"strings"

	"github.com/AndreeJait/go-utility/v2/logw"
)

// Attachment represents a file to be sent along with the email.
type Attachment struct {
	Filename string
	Content  []byte
}

// Message holds the complete data for a single email delivery.
type Message struct {
	To          []string
	Subject     string
	Body        string
	IsHTML      bool
	Attachments []Attachment
}

// Emailer defines the contract for email delivery.
type Emailer interface {
	// Send delivers the constructed Message via SMTP.
	Send(ctx context.Context, msg Message) error
}

type smtpEmailer struct {
	host     string
	port     int
	username string
	password string
	from     string
}

// New initializes a new SMTP emailer. Works with Mailtrap, Gmail, SES, etc.
func New(host string, port int, username, password, from string) Emailer {
	return &smtpEmailer{
		host:     host,
		port:     port,
		username: username,
		password: password,
		from:     from,
	}
}

// ParseTemplate takes a raw string and executes it with the provided data.
func ParseTemplate(tmpl string, data any) (string, error) {
	t, err := template.New("email").Parse(tmpl)
	if err != nil {
		return "", fmt.Errorf("emailw: failed to parse string template: %w", err)
	}
	var buf bytes.Buffer
	if err := t.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("emailw: failed to execute template: %w", err)
	}
	return buf.String(), nil
}

// ParseTemplateFile reads a file from the disk and executes it with the provided data.
func ParseTemplateFile(filePath string, data any) (string, error) {
	t, err := template.ParseFiles(filePath)
	if err != nil {
		return "", fmt.Errorf("emailw: failed to parse template file: %w", err)
	}
	var buf bytes.Buffer
	if err := t.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("emailw: failed to execute template file: %w", err)
	}
	return buf.String(), nil
}

// Send handles the complex RFC 2822 MIME multipart construction for attachments.
func (s *smtpEmailer) Send(ctx context.Context, msg Message) error {
	boundary := "boundary-1234567890"
	addr := fmt.Sprintf("%s:%d", s.host, s.port)
	auth := smtp.PlainAuth("", s.username, s.password, s.host)

	// Build Headers
	header := fmt.Sprintf("From: %s\r\n", s.from)
	header += fmt.Sprintf("To: %s\r\n", strings.Join(msg.To, ","))
	header += fmt.Sprintf("Subject: %s\r\n", msg.Subject)
	header += "MIME-Version: 1.0\r\n"
	header += fmt.Sprintf("Content-Type: multipart/mixed; boundary=%s\r\n", boundary)
	header += "\r\n"

	// Build Body
	bodyType := "text/plain"
	if msg.IsHTML {
		bodyType = "text/html"
	}

	content := fmt.Sprintf("--%s\r\n", boundary)
	content += fmt.Sprintf("Content-Type: %s; charset=\"utf-8\"\r\n\r\n", bodyType)
	content += msg.Body + "\r\n"

	// Add Attachments
	for _, att := range msg.Attachments {
		content += fmt.Sprintf("--%s\r\n", boundary)
		// Detect MIME type based on extension
		ext := filepath.Ext(att.Filename)
		mType := mime.TypeByExtension(ext)
		if mType == "" {
			mType = "application/octet-stream"
		}

		content += fmt.Sprintf("Content-Type: %s\r\n", mType)
		content += "Content-Transfer-Encoding: base64\r\n"
		content += fmt.Sprintf("Content-Disposition: attachment; filename=\"%s\"\r\n\r\n", att.Filename)

		encoded := base64.StdEncoding.EncodeToString(att.Content)
		// Splitting base64 into 76-character lines for RFC compliance
		for i := 0; i < len(encoded); i += 76 {
			end := i + 76
			if end > len(encoded) {
				end = len(encoded)
			}
			content += encoded[i:end] + "\r\n"
		}
	}
	content += fmt.Sprintf("--%s--", boundary)

	fullMsg := []byte(header + content)
	err := smtp.SendMail(addr, auth, s.from, msg.To, fullMsg)
	if err != nil {
		logw.CtxErrorf(ctx, "emailw: failed to deliver email: %v", err)
		return err
	}

	logw.CtxInfof(ctx, "emailw: successfully sent email to %v | Subject: %s", msg.To, msg.Subject)
	return nil
}
