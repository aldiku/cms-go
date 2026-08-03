package models

import (
	"encoding/json"
	"time"
)

// SMTP encryption modes for SMTPConfig.Encryption.
const (
	SMTPEncryptionNone = "none"
	SMTPEncryptionSSL  = "ssl" // implicit TLS from connect (e.g. port 465)
	SMTPEncryptionTLS  = "tls" // STARTTLS (e.g. port 587)
)

// SMTPConfig is a named outbound mail server credential set. Multiple can
// exist (e.g. one per provider); NotificationHook picks which one to send
// through. Password is stored encrypted — see utils.SimpleEncrypt and
// config.AppKey — never in plaintext.
type SMTPConfig struct {
	ID         uint   `gorm:"primaryKey"`
	Name       string `gorm:"uniqueIndex"`
	Host       string
	Port       int
	Username   string
	Password   string // encrypted; see admin_smtp.go
	Encryption string // "none" | "ssl" | "tls"
	FromEmail  string
	FromName   string
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

// EmailTemplateParam is one declared placeholder a template's Body can
// reference as "{{key}}". Stored as an element of EmailTemplate.ParamsJSON.
type EmailTemplateParam struct {
	Key         string `json:"key"`
	Description string `json:"description"`
}

// EmailTemplate is a reusable HTML email body with named placeholders.
// NotificationHook maps its own hook's data fields onto these placeholder
// keys at send time.
type EmailTemplate struct {
	ID          uint   `gorm:"primaryKey"`
	Name        string `gorm:"uniqueIndex"`
	Description string
	Subject     string
	Body        string // HTML; placeholders written as "{{param_key}}"
	ParamsJSON  string // JSON array of EmailTemplateParam
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

func (t EmailTemplate) Parameters() ([]EmailTemplateParam, error) {
	var p []EmailTemplateParam
	if t.ParamsJSON == "" {
		return p, nil
	}
	err := json.Unmarshal([]byte(t.ParamsJSON), &p)
	return p, err
}

func (t *EmailTemplate) SetParameters(p []EmailTemplateParam) error {
	b, err := json.Marshal(p)
	if err != nil {
		return err
	}
	t.ParamsJSON = string(b)
	return nil
}

// NotificationHook binds one system hook (e.g. "register_user", see
// internal/notify.Registry) to an SMTPConfig + EmailTemplate pair, with a
// mapping from the template's declared parameter keys to the hook's
// available data field keys. Multiple bindings can share the same HookKey
// (e.g. a "welcome email" and an "admin alert" both firing on register_user).
type NotificationHook struct {
	ID              uint   `gorm:"primaryKey"`
	HookKey         string `gorm:"index"`
	Name            string
	Status          uint8 // 1 = active
	SMTPConfigID    uint
	SMTPConfig      SMTPConfig `gorm:"foreignKey:SMTPConfigID"`
	EmailTemplateID uint
	EmailTemplate   EmailTemplate `gorm:"foreignKey:EmailTemplateID"`
	MappingJSON     string        // JSON object: template param key -> hook field key
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

func (h NotificationHook) Mapping() (map[string]string, error) {
	m := map[string]string{}
	if h.MappingJSON == "" {
		return m, nil
	}
	err := json.Unmarshal([]byte(h.MappingJSON), &m)
	return m, err
}

func (h *NotificationHook) SetMapping(m map[string]string) error {
	b, err := json.Marshal(m)
	if err != nil {
		return err
	}
	h.MappingJSON = string(b)
	return nil
}
