package mailer

import (
	"bytes"
	"fmt"
	"strings"
	"text/template"
)

type IdentityView struct {
	First     string
	Middle    string
	Last      string
	DOB       string
	Emails    []string
	Phones    []string
	Addresses []string
	Cities    []string
	Usernames []string
	Aliases   []string
	Relatives []string
}

func (v IdentityView) FullName() string {
	parts := []string{v.First}
	if v.Middle != "" {
		parts = append(parts, v.Middle)
	}
	parts = append(parts, v.Last)
	return strings.TrimSpace(strings.Join(parts, " "))
}

func (v IdentityView) PrimaryEmail() string {
	if len(v.Emails) == 0 {
		return ""
	}
	return v.Emails[0]
}

// Fields is the template execution context.
type Fields struct {
	BrokerName   string
	BrokerDomain string
	FullName     string
	DateOfBirth  string
	Emails       string
	Phones       string
	Addresses    string
	Usernames    string
	TrackingID   string
	PrimaryEmail string
	StatuteName  string
}

func viewToFields(brokerName, brokerDomain, tracking string, id IdentityView, statute string) Fields {
	dash := func(ss []string) string {
		if len(ss) == 0 {
			return "(none provided)"
		}
		return strings.Join(ss, ", ")
	}
	dob := id.DOB
	if dob == "" {
		dob = "(not provided)"
	}
	return Fields{
		BrokerName:   brokerName,
		BrokerDomain: brokerDomain,
		FullName:     id.FullName(),
		DateOfBirth:  dob,
		Emails:       dash(id.Emails),
		Phones:       dash(id.Phones),
		Addresses:    dash(id.Addresses),
		Usernames:    dash(id.Usernames),
		TrackingID:   tracking,
		PrimaryEmail: id.PrimaryEmail(),
		StatuteName:  statute,
	}
}

func render(name, body string, f Fields) (subject, text string, err error) {
	tmpl, err := template.New(name).Parse(body)
	if err != nil {
		return "", "", fmt.Errorf("parse template %s: %w", name, err)
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, f); err != nil {
		return "", "", fmt.Errorf("execute template %s: %w", name, err)
	}
	raw := buf.String()
	subj := ""
	rest := raw
	if strings.HasPrefix(raw, "Subject:") {
		line, after, _ := strings.Cut(raw, "\n")
		subj = strings.TrimSpace(strings.TrimPrefix(line, "Subject:"))
		rest = strings.TrimLeft(after, "\r\n")
	}
	return subj, rest, nil
}
