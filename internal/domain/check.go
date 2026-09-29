package domain

import (
	"fmt"
	"math"
	"strings"
	"unicode/utf8"
)

func CheckLabel(value, field string, max int) error {
	value = strings.TrimSpace(value)
	if value == "" {
		return fmt.Errorf("%s is required", field)
	}
	if utf8.RuneCountInString(value) > max {
		return fmt.Errorf("%s must be at most %d characters", field, max)
	}
	for _, r := range value {
		if r < 32 || r == 127 {
			return fmt.Errorf("%s contains invalid characters", field)
		}
	}
	return nil
}

func CheckOptional(value, field string, max int) error {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return CheckLabel(value, field, max)
}

func CheckUsername(value string) error {
	value = strings.TrimSpace(value)
	if utf8.RuneCountInString(value) < 3 || utf8.RuneCountInString(value) > 64 {
		return fmt.Errorf("username must be 3 to 64 characters")
	}
	for i, r := range value {
		ok := (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '.' || r == '_' || r == '-'
		if !ok || (i == 0 && (r == '.' || r == '_' || r == '-')) {
			return fmt.Errorf("username may contain letters, numbers, dots, underscores, and hyphens")
		}
	}
	return nil
}

func CheckEmail(value string) error {
	value = strings.TrimSpace(value)
	if utf8.RuneCountInString(value) > 254 || !strings.Contains(value, "@") || strings.ContainsAny(value, " \t") {
		return fmt.Errorf("email is not valid")
	}
	local, host, ok := strings.Cut(value, "@")
	if !ok || local == "" || strings.HasPrefix(host, ".") || strings.HasSuffix(host, ".") {
		return fmt.Errorf("email is not valid")
	}
	if host != "localhost" && !strings.Contains(host, ".") {
		return fmt.Errorf("email is not valid")
	}
	return nil
}

func CheckPassword(password, username string) error {
	if utf8.RuneCountInString(password) < 10 || utf8.RuneCountInString(password) > 200 {
		return fmt.Errorf("password must be 10 to 200 characters")
	}
	if strings.TrimSpace(password) == "" {
		return fmt.Errorf("password must be 10 to 200 characters")
	}
	if username != "" && strings.EqualFold(password, username) {
		return fmt.Errorf("password must not match the username")
	}
	return nil
}

func CheckRole(role string) error {
	if Rank(role) == 0 && role != RoleViewer {
		return fmt.Errorf("role is not valid")
	}
	switch role {
	case RoleViewer, RoleOperator, RoleInspector, RoleSupervisor, RoleQualityManager, RoleAdmin:
		return nil
	default:
		return fmt.Errorf("role is not valid")
	}
}

func CheckOneOf(value, field string, allowed []string) error {
	for _, a := range allowed {
		if value == a {
			return nil
		}
	}
	return fmt.Errorf("%s must be one of: %s", field, strings.Join(allowed, ", "))
}

func CheckFinite(v *float64, field string) error {
	if v == nil {
		return nil
	}
	if math.IsNaN(*v) || math.IsInf(*v, 0) {
		return fmt.Errorf("%s must be a finite number", field)
	}
	return nil
}

func CheckPositive(v float64, field string) error {
	if math.IsNaN(v) || math.IsInf(v, 0) || v <= 0 || v > 1e12 {
		return fmt.Errorf("%s must be greater than 0", field)
	}
	return nil
}

func CheckNonNegative(v float64, field string) error {
	if math.IsNaN(v) || math.IsInf(v, 0) || v < 0 || v > 1e12 {
		return fmt.Errorf("%s must be zero or greater", field)
	}
	return nil
}

// Trim collapses the surrounding space used by every text column.
func Trim(s string) string {
	return strings.TrimSpace(s)
}
