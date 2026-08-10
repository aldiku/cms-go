package paymentgateway

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"cms-go/internal/models"
	"cms-go/internal/notify"
)

var httpClient = &http.Client{Timeout: 15 * time.Second}

// createGenericCharge opens a charge through any gateway configured purely
// via PaymentGateway's RequestPath/RequestBodyTemplate/ResponseFieldMap —
// no per-provider Go code. BaseURL+RequestPath is POSTed with HTTP Basic
// Auth (decrypted APIKey as username, empty password — the convention
// shared by Midtrans and Xendit alike) and RequestBodyTemplate rendered
// against req's fields via notify.Render (the same {{key}} engine
// NotificationHook templates use); the response is walked per
// ResponseFieldMap's dot-paths into a normalized ChargeResponse.
func createGenericCharge(gw models.PaymentGateway, req ChargeRequest) (ChargeResponse, error) {
	apiKey, err := decryptAPIKey(gw)
	if err != nil {
		return ChargeResponse{}, err
	}
	if gw.BaseURL == "" {
		return ChargeResponse{}, fmt.Errorf("payment gateway %q has no base URL configured", gw.Name)
	}
	if gw.RequestBodyTemplate == "" {
		return ChargeResponse{}, fmt.Errorf("payment gateway %q has no request body template configured", gw.Name)
	}

	params := map[string]string{
		"order_id":       req.OrderID,
		"amount":         strconv.FormatInt(req.Amount, 10),
		"bank_code":      req.BankCode,
		"customer_name":  req.CustomerName,
		"customer_email": req.CustomerEmail,
	}
	body := notify.Render(gw.RequestBodyTemplate, params)

	url := strings.TrimRight(gw.BaseURL, "/") + gw.RequestPath
	httpReq, err := http.NewRequest(http.MethodPost, url, bytes.NewReader([]byte(body)))
	if err != nil {
		return ChargeResponse{}, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json")
	httpReq.Header.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte(apiKey+":")))

	resp, err := httpClient.Do(httpReq)
	if err != nil {
		return ChargeResponse{}, fmt.Errorf("gateway request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return ChargeResponse{}, err
	}

	if resp.StatusCode >= 300 {
		return ChargeResponse{Raw: string(respBody)}, fmt.Errorf("gateway returned HTTP %d: %s", resp.StatusCode, string(respBody))
	}

	var parsed interface{}
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return ChargeResponse{Raw: string(respBody)}, fmt.Errorf("gateway returned an unparseable response (HTTP %d)", resp.StatusCode)
	}

	fieldMap := map[string]string{}
	if gw.ResponseFieldMap != "" {
		json.Unmarshal([]byte(gw.ResponseFieldMap), &fieldMap)
	}

	result := ChargeResponse{Raw: string(respBody)}
	result.VANumber = extractPath(parsed, fieldMap["va_number"])
	result.BankCode = extractPath(parsed, fieldMap["bank_code"])
	result.PaymentURL = extractPath(parsed, fieldMap["payment_url"])
	result.GatewayReferenceID = extractPath(parsed, fieldMap["reference_id"])
	if expiresRaw := extractPath(parsed, fieldMap["expires_at"]); expiresRaw != "" {
		for _, layout := range []string{time.RFC3339, "2006-01-02 15:04:05", "2006-01-02T15:04:05"} {
			if t, err := time.Parse(layout, expiresRaw); err == nil {
				result.ExpiresAt = &t
				break
			}
		}
	}
	return result, nil
}

// extractPath walks a dot-separated path (numeric segments index arrays,
// e.g. "va_numbers.0.va_number") through a decoded JSON value. Returns ""
// if path is empty or any segment doesn't resolve — a missing/misconfigured
// mapping degrades to an empty field, not a crash.
func extractPath(v interface{}, path string) string {
	if path == "" {
		return ""
	}
	cur := v
	for _, seg := range strings.Split(path, ".") {
		switch node := cur.(type) {
		case map[string]interface{}:
			cur = node[seg]
		case []interface{}:
			idx, err := strconv.Atoi(seg)
			if err != nil || idx < 0 || idx >= len(node) {
				return ""
			}
			cur = node[idx]
		default:
			return ""
		}
	}
	switch val := cur.(type) {
	case string:
		return val
	case float64:
		return strconv.FormatFloat(val, 'f', -1, 64)
	case nil:
		return ""
	default:
		b, _ := json.Marshal(val)
		return string(b)
	}
}
