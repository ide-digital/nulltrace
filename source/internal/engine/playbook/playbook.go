package playbook

import (
	"strconv"
	"strings"

	"github.com/4x3/nulltrace/internal/broker"
)

type Entry struct {
	BrokerID   string `json:"broker_id"`
	BrokerName string `json:"broker_name"`
	Domain     string `json:"domain"`
	OptOutURL  string `json:"opt_out_url"`
	Email      string `json:"contact_email,omitempty"`
	Captcha    bool   `json:"requires_captcha"`
	Steps      []Step `json:"steps"`
}

type Step struct {
	Title string `json:"title"`
	Body  string `json:"body"`
}

func For(b broker.Broker) Entry {
	e := Entry{
		BrokerID:   b.ID,
		BrokerName: b.Name,
		Domain:     b.Domain,
		OptOutURL:  b.OptOutURL,
		Email:      b.ContactEmail,
		Captcha:    b.RequiresCaptcha,
	}
	if b.OptOutURL != "" {
		e.Steps = append(e.Steps, Step{
			Title: "Open the opt-out page",
			Body:  b.OptOutURL,
		})
	}
	body := strings.TrimSpace(b.Playbook)
	if body == "" {
		body = "Fill the broker's form with the identity in the vault. If a captcha appears, finish it in the browser."
	}
	e.Steps = append(e.Steps, Step{Title: "Submit the request", Body: body})
	if b.RequiresEmailConfirmation {
		e.Steps = append(e.Steps, Step{
			Title: "Watch for a confirm email",
			Body:  "IMAP will log the link. Turn on imap.auto_fetch if you want a plain GET of confirmation URLs.",
		})
	}
	e.Steps = append(e.Steps, Step{
		Title: "Mark it verified when the listing is gone",
		Body:  "nulltrace action verify <record-id>. After ~45 days the watchdog asks you to check again.",
	})
	return e
}

func AllManual(brokers []broker.Broker) []Entry {
	var out []Entry
	for _, b := range brokers {
		if b.RequiresHuman() {
			out = append(out, For(b))
		}
	}
	return out
}

func Render(e Entry) string {
	var b strings.Builder
	b.WriteString(e.BrokerName)
	if e.Domain != "" {
		b.WriteString(" (")
		b.WriteString(e.Domain)
		b.WriteString(")")
	}
	b.WriteByte('\n')
	if e.OptOutURL != "" {
		b.WriteString("URL: ")
		b.WriteString(e.OptOutURL)
		b.WriteByte('\n')
	}
	if e.Email != "" {
		b.WriteString("Privacy email: ")
		b.WriteString(e.Email)
		b.WriteByte('\n')
	}
	if e.Captcha {
		b.WriteString("Captcha: expected\n")
	}
	for i, s := range e.Steps {
		b.WriteString("\n")
		b.WriteString(strconv.Itoa(i + 1))
		b.WriteString(". ")
		b.WriteString(s.Title)
		b.WriteString("\n   ")
		b.WriteString(s.Body)
		b.WriteByte('\n')
	}
	return b.String()
}
