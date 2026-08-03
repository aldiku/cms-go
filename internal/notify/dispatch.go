package notify

import (
	"fmt"

	"cms-go/internal/config"
	"cms-go/internal/db"
	"cms-go/internal/models"
	"cms-go/internal/utils"
)

// Dispatch sends notifications for every active NotificationHook bound to
// hookKey. data is the hook's field values for this firing (see the
// relevant notify.HookDef.Fields for the keys a given hook exposes), plus a
// required "_to" entry — the recipient address — which is not itself
// mappable to a template parameter, only the other keys are. Returns one
// error per failed binding; nil/empty means every bound notification sent
// (or none were bound, which is not an error — an unbound hook just does
// nothing).
func Dispatch(hookKey string, data map[string]string) []error {
	var hooks []models.NotificationHook
	db.DB.Preload("SMTPConfig").Preload("EmailTemplate").
		Where("hook_key = ? AND status = 1", hookKey).Find(&hooks)

	to := data["_to"]
	var errs []error
	for _, h := range hooks {
		if err := deliver(h, to, data); err != nil {
			errs = append(errs, fmt.Errorf("notification hook %d (%s): %w", h.ID, h.Name, err))
		}
	}
	return errs
}

// SendTest renders h's template against data and sends it to `to`,
// bypassing Dispatch's data["_to"] convention — used by the Notification
// Manager's "Send test" action, where the admin supplies both the
// recipient and (typically sample) field values directly.
func SendTest(h models.NotificationHook, to string, data map[string]string) error {
	return deliver(h, to, data)
}

func deliver(h models.NotificationHook, to string, data map[string]string) error {
	if to == "" {
		return fmt.Errorf("no recipient address")
	}

	mapping, err := h.Mapping()
	if err != nil {
		return fmt.Errorf("invalid param mapping: %w", err)
	}

	params := make(map[string]string, len(mapping))
	for templateParam, hookField := range mapping {
		params[templateParam] = data[hookField]
	}

	password, err := utils.SimpleDecrypt(h.SMTPConfig.Password, config.AppKey())
	if err != nil {
		return fmt.Errorf("decrypt smtp password: %w", err)
	}

	subject := Render(h.EmailTemplate.Subject, params)
	body := Render(h.EmailTemplate.Body, params)
	return Send(h.SMTPConfig, password, to, subject, body)
}
