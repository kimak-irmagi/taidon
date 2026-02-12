package app

import "testing"

func TestSplitConfigAssignment(t *testing.T) {
	if _, _, ok := splitConfigAssignment(""); ok {
		t.Fatalf("expected empty input to fail")
	}
	if _, _, ok := splitConfigAssignment("a"); ok {
		t.Fatalf("expected missing equals to fail")
	}
	if _, _, ok := splitConfigAssignment("a="); ok {
		t.Fatalf("expected missing value to fail")
	}
	if _, _, ok := splitConfigAssignment("=b"); ok {
		t.Fatalf("expected missing path to fail")
	}

	path, value, ok := splitConfigAssignment(" log.level = info ")
	if !ok || path != "log.level" || value != "info" {
		t.Fatalf("unexpected assignment parse: %q %q ok=%v", path, value, ok)
	}
}

func TestAutoQuoteJSONValue(t *testing.T) {
	if value, ok := autoQuoteJSONValue("info"); !ok || value != "info" {
		t.Fatalf("expected bareword to auto-quote, got %v ok=%v", value, ok)
	}
	if _, ok := autoQuoteJSONValue("true"); ok {
		t.Fatalf("expected boolean to skip auto-quote")
	}
	if _, ok := autoQuoteJSONValue("null"); ok {
		t.Fatalf("expected null to skip auto-quote")
	}
	if _, ok := autoQuoteJSONValue("1"); ok {
		t.Fatalf("expected numeric to skip auto-quote")
	}
	if _, ok := autoQuoteJSONValue("with space"); ok {
		t.Fatalf("expected spaced value to skip auto-quote")
	}
}

func TestLooksLikeJSONValue(t *testing.T) {
	cases := map[string]bool{
		"{": true,
		"[": true,
		"\"": true,
		"1": true,
		"-": true,
		"a": false,
	}
	for input, expected := range cases {
		if got := looksLikeJSONValue(input); got != expected {
			t.Fatalf("looksLikeJSONValue(%q)=%v, want %v", input, got, expected)
		}
	}
}

func TestIsBareword(t *testing.T) {
	if !isBareword("alpha-1_2.beta") {
		t.Fatalf("expected bareword to be allowed")
	}
	if isBareword("with space") {
		t.Fatalf("expected space to be rejected")
	}
	if isBareword("with/slash") {
		t.Fatalf("expected slash to be rejected")
	}
}
