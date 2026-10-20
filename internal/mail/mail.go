package mail

import (
	"fmt"
	"net/smtp"
	"os"
	"strings"
)

// Configured reports whether YARD_SMTP_HOST is set.
func Configured() bool {
	return os.Getenv("YARD_SMTP_HOST") != ""
}

// Send delivers a plain-text message using YARD_SMTP_HOST/PORT/USER/PASS/FROM.
func Send(to, subject, body string) error {
	host := os.Getenv("YARD_SMTP_HOST")
	if host == "" {
		return fmt.Errorf("smtp not configured")
	}
	port := os.Getenv("YARD_SMTP_PORT")
	if port == "" {
		port = "587"
	}
	from := os.Getenv("YARD_SMTP_FROM")
	if from == "" {
		from = "yard@localhost"
	}
	msg, err := buildMessage(from, to, subject, body)
	if err != nil {
		return err
	}
	var auth smtp.Auth
	if user := os.Getenv("YARD_SMTP_USER"); user != "" {
		auth = smtp.PlainAuth("", user, os.Getenv("YARD_SMTP_PASS"), host)
	}
	return smtp.SendMail(host+":"+port, auth, from, []string{to}, msg)
}

// buildMessage assembles the RFC 5322 message, refusing CR/LF in addresses and
// flattening them in the subject so callers cannot inject extra headers.
func buildMessage(from, to, subject, body string) ([]byte, error) {
	if strings.ContainsAny(to, "\r\n") || strings.ContainsAny(from, "\r\n") {
		return nil, fmt.Errorf("invalid address")
	}
	subject = strings.NewReplacer("\r", " ", "\n", " ").Replace(subject)
	return []byte(fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: %s\r\n\r\n%s", from, to, subject, body)), nil
}
