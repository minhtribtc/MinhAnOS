package apicontract

import "net/http"

// Code is independent of translated legacy messages. Narrow domain codes may be added compatibly.
func Code(status int) string {
	switch status {
	case 400:
		return "INVALID_REQUEST"
	case 401:
		return "AUTH_REQUIRED"
	case 403:
		return "ACCESS_DENIED"
	case 404:
		return "NOT_FOUND"
	case 405:
		return "METHOD_NOT_ALLOWED"
	case 409:
		return "CONFLICT"
	case 413:
		return "PAYLOAD_TOO_LARGE"
	case 415:
		return "UNSUPPORTED_MEDIA_TYPE"
	case 422:
		return "RESOURCE_LIMIT"
	case 507:
		return "STORAGE_FULL"
	case 429:
		return "RATE_LIMITED"
	case 503:
		return "SERVICE_UNAVAILABLE"
	default:
		if status >= http.StatusInternalServerError {
			return "INTERNAL_ERROR"
		}
		return "REQUEST_FAILED"
	}
}
func Error(status int, message string, code ...string) map[string]string {
	c := Code(status)
	if len(code) > 0 {
		c = code[0]
	}
	return map[string]string{"code": c, "error": message}
}
