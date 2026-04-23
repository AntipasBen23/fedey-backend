package utils

import (
	"crypto/tls"
	"fmt"
	"net/smtp"
	"os"
)

// SendEmail sends an HTML email via SMTP
func SendEmail(to string, subject string, body string) error {
	host := os.Getenv("SMTP_HOST") // e.g. smtp.gmail.com
	port := os.Getenv("SMTP_PORT") // e.g. 587
	user := os.Getenv("SMTP_USER")
	pass := os.Getenv("SMTP_PASS")

	if host == "" || port == "" || user == "" || pass == "" {
		return fmt.Errorf("SMTP configuration missing in environment variables")
	}

	auth := smtp.PlainAuth("", user, pass, host)

	// Build the message
	mime := "MIME-version: 1.0;\nContent-Type: text/html; charset=\"UTF-8\";\n\n"
	subjectHeader := "Subject: " + subject + "\n"
	msg := []byte(subjectHeader + mime + body)

	// Gmail requires TLS
	addr := host + ":" + port
	
	// Use standard StartTLS
	config := &tls.Config{InsecureSkipVerify: true, ServerName: host}
	
	c, err := smtp.Dial(addr)
	if err != nil {
		return err
	}
	defer c.Close()

	if err = c.StartTLS(config); err != nil {
		return err
	}

	if err = c.Auth(auth); err != nil {
		return err
	}

	if err = c.Mail(user); err != nil {
		return err
	}

	if err = c.Rcpt(to); err != nil {
		return err
	}

	w, err := c.Data()
	if err != nil {
		return err
	}

	_, err = w.Write(msg)
	if err != nil {
		return err
	}

	err = w.Close()
	if err != nil {
		return err
	}

	return c.Quit()
}
