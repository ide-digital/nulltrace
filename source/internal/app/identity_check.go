package app

import (
	"bufio"
	"errors"
	"fmt"
	"net/mail"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/4x3/nulltrace/internal/broker"
	"github.com/4x3/nulltrace/internal/tui"
)

func promptChecked(in *bufio.Reader, label string, required bool, check func(string) error) string {
	for {
		v := promptLine(in, label)
		if v == "" {
			if required {
				fmt.Println("  " + tui.Warn("this field is required"))
				continue
			}
			return ""
		}
		if err := check(v); err != nil {
			fmt.Println("  " + tui.Warn(err.Error()))
			continue
		}
		return v
	}
}

func promptCity(in *bufio.Reader, label string) string {
	for {
		v := promptLine(in, label)
		if v == "" {
			return ""
		}
		if err := checkCityShape(v); err != nil {
			fmt.Println("  " + tui.Warn(err.Error()))
			continue
		}
		if !knownUSCity(v) {
			fmt.Println("  " + tui.Warn(fmt.Sprintf("%q is not on the US city list", v)))
			if !promptYes(in, "  keep it anyway", false) {
				continue
			}
		}
		return v
	}
}

func checkPersonName(s string, allowInitial bool) error {
	s = strings.TrimSpace(s)
	letters := 0
	for _, r := range s {
		switch {
		case unicode.IsLetter(r):
			letters++
		case r == ' ' || r == '-' || r == '\'' || r == '.' || r == '’':
			// ok
		default:
			return fmt.Errorf("use letters only (hyphens and apostrophes are fine)")
		}
	}
	if letters == 0 {
		return fmt.Errorf("that doesn't look like a name")
	}
	if allowInitial && letters == 1 && utf8.RuneCountInString(s) <= 2 {
		return nil
	}
	if letters < 2 {
		return fmt.Errorf("name is too short")
	}
	return nil
}

func checkEmail(s string) error {
	s = strings.TrimSpace(s)
	if strings.ContainsAny(s, " \t") {
		return fmt.Errorf("email can't contain spaces")
	}
	addr, err := mail.ParseAddress(s)
	if err != nil || addr.Address == "" {
		return fmt.Errorf("need a real email, like name@example.com")
	}
	at := strings.LastIndex(addr.Address, "@")
	if at < 1 || at == len(addr.Address)-1 {
		return fmt.Errorf("need a real email, like name@example.com")
	}
	domain := addr.Address[at+1:]
	if !strings.Contains(domain, ".") || strings.HasPrefix(domain, ".") || strings.HasSuffix(domain, ".") {
		return fmt.Errorf("email domain looks incomplete")
	}
	dot := strings.LastIndex(domain, ".")
	if len(domain[dot+1:]) < 2 {
		return fmt.Errorf("email domain looks incomplete")
	}
	return nil
}

func checkPhone(s string) error {
	var digits strings.Builder
	for _, r := range s {
		switch {
		case r >= '0' && r <= '9':
			digits.WriteRune(r)
		case r == ' ' || r == '-' || r == '(' || r == ')' || r == '.' || r == '+':
			// formatting
		default:
			return fmt.Errorf("need a phone number with 10 digits, or +country code")
		}
	}
	n := digits.String()
	if len(n) == 11 && n[0] == '1' {
		n = n[1:]
	}
	if len(n) == 10 {
		return nil
	}
	if len(n) >= 8 && len(n) <= 15 {
		return nil
	}
	return fmt.Errorf("need a phone number with 10 digits, or +country code")
}

func checkDOB(s string) error {
	t, ok := parseDOB(s)
	if !ok {
		return fmt.Errorf("use a date like 1991-04-23 or 04/23/1991")
	}
	now := time.Now()
	if t.After(now) {
		return fmt.Errorf("date of birth is in the future")
	}
	if t.Before(now.AddDate(-120, 0, 0)) {
		return fmt.Errorf("date of birth is too far in the past")
	}
	if t.After(now.AddDate(-13, 0, 0)) {
		return fmt.Errorf("date of birth is too recent")
	}
	return nil
}

func parseDOB(s string) (time.Time, bool) {
	s = strings.TrimSpace(s)
	layouts := []string{
		"2006-01-02",
		"01/02/2006",
		"1/2/2006",
		"01-02-2006",
		"1-2-2006",
		"January 2, 2006",
		"Jan 2, 2006",
		"2 January 2006",
		"2 Jan 2006",
	}
	for _, layout := range layouts {
		if t, err := time.Parse(layout, s); err == nil {
			return t, true
		}
	}
	if len(s) == 8 {
		if _, err := strconv.Atoi(s); err == nil {
			if t, err := time.Parse("20060102", s); err == nil {
				return t, true
			}
		}
	}
	return time.Time{}, false
}

func checkCityShape(s string) error {
	s = strings.TrimSpace(s)
	letters := 0
	for _, r := range s {
		switch {
		case unicode.IsLetter(r):
			letters++
		case r == ' ' || r == '-' || r == '\'' || r == '.' || r == '’':
			// ok
		default:
			return fmt.Errorf("city should be a place name, like Austin or New York")
		}
	}
	if letters < 3 {
		return fmt.Errorf("city name is too short")
	}
	return nil
}

func checkAddress(s string) error {
	s = strings.TrimSpace(s)
	hasLetter, hasDigit := false, false
	for _, r := range s {
		if unicode.IsLetter(r) {
			hasLetter = true
		}
		if unicode.IsDigit(r) {
			hasDigit = true
		}
	}
	if !hasLetter || !hasDigit || utf8.RuneCountInString(s) < 5 {
		return fmt.Errorf("need a street address, like 123 Main St")
	}
	return nil
}

func checkUsername(s string) error {
	s = strings.TrimSpace(s)
	if utf8.RuneCountInString(s) < 2 {
		return fmt.Errorf("username is too short")
	}
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' || r == '.' || r == '-' {
			continue
		}
		return fmt.Errorf("username can only use letters, numbers, dots, and underscores")
	}
	return nil
}

func checkAttrValue(typ, value string) error {
	value = strings.TrimSpace(value)
	if value == "" {
		return errors.New("value is empty")
	}
	switch strings.ToUpper(typ) {
	case string(broker.AttrEmail):
		return checkEmail(value)
	case string(broker.AttrPhone):
		return checkPhone(value)
	case string(broker.AttrCity):
		return checkCityShape(value)
	case string(broker.AttrAddress):
		return checkAddress(value)
	case string(broker.AttrUsername), string(broker.AttrAlias):
		return checkUsername(value)
	default:
		if utf8.RuneCountInString(value) < 2 {
			return fmt.Errorf("value is too short")
		}
		return nil
	}
}
