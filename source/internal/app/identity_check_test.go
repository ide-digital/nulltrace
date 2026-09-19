package app

import "testing"

func TestCheckEmail(t *testing.T) {
	if err := checkEmail("fe"); err == nil {
		t.Fatal("bare letters should fail")
	}
	if err := checkEmail("john@gmail.com"); err != nil {
		t.Fatal(err)
	}
	if err := checkEmail("not an email"); err == nil {
		t.Fatal("spaces should fail")
	}
}

func TestCheckDOB(t *testing.T) {
	if err := checkDOB("fe"); err == nil {
		t.Fatal("garbage should fail")
	}
	if err := checkDOB("1991-04-23"); err != nil {
		t.Fatal(err)
	}
	if err := checkDOB("04/23/1991"); err != nil {
		t.Fatal(err)
	}
	if err := checkDOB("2099-01-01"); err == nil {
		t.Fatal("future dates should fail")
	}
	if err := checkDOB("2024-01-01"); err == nil {
		t.Fatal("recent dates should fail")
	}
}

func TestCheckPhone(t *testing.T) {
	if err := checkPhone("fe"); err == nil {
		t.Fatal("letters should fail")
	}
	if err := checkPhone("555-123-4567"); err != nil {
		t.Fatal(err)
	}
	if err := checkPhone("+1 (415) 555-0100"); err != nil {
		t.Fatal(err)
	}
	if err := checkPhone("abc5551234567"); err == nil {
		t.Fatal("letters mixed into a number should fail")
	}
}

func TestCheckName(t *testing.T) {
	if err := checkPersonName("john", false); err != nil {
		t.Fatal(err)
	}
	if err := checkPersonName("O'Neil", false); err != nil {
		t.Fatal(err)
	}
	if err := checkPersonName("12", false); err == nil {
		t.Fatal("digits should fail")
	}
	if err := checkPersonName("J", true); err != nil {
		t.Fatal(err)
	}
}

func TestCheckCity(t *testing.T) {
	if err := checkCityShape("fe"); err == nil {
		t.Fatal("two letters should fail")
	}
	if err := checkCityShape("Austin"); err != nil {
		t.Fatal(err)
	}
	if !knownUSCity("New York") || !knownUSCity("st. louis") || !knownUSCity("Los Angeles") {
		t.Fatal("expected known US cities")
	}
	if knownUSCity("fe") {
		t.Fatal("fe is not a city")
	}
}

func TestCheckAddress(t *testing.T) {
	if err := checkAddress("fe"); err == nil {
		t.Fatal("should fail")
	}
	if err := checkAddress("123 Main St"); err != nil {
		t.Fatal(err)
	}
}
