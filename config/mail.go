package config

import (
	"fmt"
	"net/smtp"
	"os"
)

// SendMail mengirim email sederhana (plain text). Konfigurasi via env vars:
// SMTP_HOST, SMTP_PORT, SMTP_USER, SMTP_PASS, FROM_EMAIL
func SendMail(to string, subject string, body string) error {
	host := os.Getenv("SMTP_HOST")
	port := os.Getenv("SMTP_PORT")
	user := os.Getenv("SMTP_USER")
	pass := os.Getenv("SMTP_PASS")
	from := os.Getenv("FROM_EMAIL")

	if host == "" || port == "" || user == "" || pass == "" || from == "" {
		// SMTP not configured; log to stdout for dev
		fmt.Printf("[mail] SMTP not configured. To: %s Subject: %s Body:\n%s\n", to, subject, body)
		return nil
	}

	addr := fmt.Sprintf("%s:%s", host, port)
	auth := smtp.PlainAuth("", user, pass, host)

	msg := []byte(fmt.Sprintf("To: %s\r\nSubject: %s\r\nMIME-version: 1.0;\r\nContent-Type: text/plain; charset=UTF-8;\r\n\r\n%s", to, subject, body))
	if err := smtp.SendMail(addr, auth, from, []string{to}, msg); err != nil {
		return err
	}
	return nil
}
