package notify

import (
	"crypto/tls"
	"fmt"
	"net/smtp"
	"strings"

	"cms-go/internal/models"
)

// Send delivers a single HTML email through cfg. password is passed
// separately (already decrypted by the caller) rather than read off cfg, so
// this package never has to know how/whether passwords are encrypted at
// rest.
func Send(cfg models.SMTPConfig, password, to, subject, htmlBody string) error {
	addr := fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)

	from := cfg.FromEmail
	if cfg.FromName != "" {
		from = fmt.Sprintf("%s <%s>", cfg.FromName, cfg.FromEmail)
	}

	var msg strings.Builder
	writeHeader := func(k, v string) { msg.WriteString(k + ": " + v + "\r\n") }
	writeHeader("From", from)
	writeHeader("To", to)
	writeHeader("Subject", subject)
	writeHeader("MIME-Version", "1.0")
	writeHeader("Content-Type", `text/html; charset="UTF-8"`)
	msg.WriteString("\r\n")
	msg.WriteString(htmlBody)

	var auth smtp.Auth
	if cfg.Username != "" {
		auth = smtp.PlainAuth("", cfg.Username, password, cfg.Host)
	}

	if cfg.Encryption == models.SMTPEncryptionSSL {
		return sendImplicitTLS(addr, cfg.Host, auth, cfg.FromEmail, to, []byte(msg.String()))
	}
	// "tls" (STARTTLS) and "none" both start as a plaintext connection;
	// smtp.SendMail negotiates STARTTLS itself when the server offers it,
	// which covers the "tls" case, and simply skips it for "none".
	return smtp.SendMail(addr, auth, cfg.FromEmail, []string{to}, []byte(msg.String()))
}

// sendImplicitTLS handles servers that expect TLS from the very first byte
// of the connection (traditionally port 465) — smtp.SendMail can't do this,
// it only ever starts plaintext and optionally upgrades via STARTTLS.
func sendImplicitTLS(addr, host string, auth smtp.Auth, from, to string, msg []byte) error {
	conn, err := tls.Dial("tcp", addr, &tls.Config{ServerName: host})
	if err != nil {
		return fmt.Errorf("tls dial: %w", err)
	}
	defer conn.Close()

	client, err := smtp.NewClient(conn, host)
	if err != nil {
		return fmt.Errorf("smtp handshake: %w", err)
	}
	defer client.Close()

	if auth != nil {
		if err := client.Auth(auth); err != nil {
			return fmt.Errorf("smtp auth: %w", err)
		}
	}
	if err := client.Mail(from); err != nil {
		return fmt.Errorf("MAIL FROM: %w", err)
	}
	if err := client.Rcpt(to); err != nil {
		return fmt.Errorf("RCPT TO: %w", err)
	}
	w, err := client.Data()
	if err != nil {
		return fmt.Errorf("DATA: %w", err)
	}
	if _, err := w.Write(msg); err != nil {
		return err
	}
	if err := w.Close(); err != nil {
		return err
	}
	return client.Quit()
}
