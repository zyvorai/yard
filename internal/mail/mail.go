package mail

import (
	"fmt"
	"net/smtp"
	"os"
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
	msg := []byte(fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: %s\r\n\r\n%s", from, to, subject, body))
	var auth smtp.Auth
	if user := os.Getenv("YARD_SMTP_USER"); user != "" {
		auth = smtp.PlainAuth("", user, os.Getenv("YARD_SMTP_PASS"), host)
	}
	return smtp.SendMail(host+":"+port, auth, from, []string{to}, msg)
}
