package broker

import "testing"

func TestLoadEmbeddedRegistry(t *testing.T) {
	reg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if reg.Len() < 50 {
		t.Fatalf("expected a populated registry, got %d", reg.Len())
	}
	if _, ok := reg.Get("spokeo"); !ok {
		t.Fatal("missing spokeo")
	}
	if _, ok := reg.Get("acxiom"); !ok {
		t.Fatal("missing acxiom")
	}
	spokeo, _ := reg.Get("spokeo")
	if spokeo.ContactEmail == "" {
		t.Fatal("spokeo should pick up privacy email from email_contacts.yaml")
	}
	if spokeo.RequiresCaptcha != true {
		t.Fatal("spokeo captcha flag should remain informational")
	}
	ids := map[string]struct{}{}
	domains := map[string]struct{}{}
	for _, b := range reg.All() {
		if _, dup := ids[b.ID]; dup {
			t.Fatalf("duplicate id %s", b.ID)
		}
		ids[b.ID] = struct{}{}
		if b.Domain == "" {
			t.Fatalf("broker %s missing domain", b.ID)
		}
		if _, dup := domains[b.Domain]; dup {
			t.Fatalf("duplicate domain %s (%s)", b.Domain, b.ID)
		}
		domains[b.Domain] = struct{}{}
	}
}
