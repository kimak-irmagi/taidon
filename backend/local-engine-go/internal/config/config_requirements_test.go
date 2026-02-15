package config

import "testing"

func TestValidateValueLogLevelVariants(t *testing.T) {
	if err := validateValue("log.level", nil); err != nil {
		t.Fatalf("expected nil log.level to be allowed, got %v", err)
	}
	if err := validateValue("log.level", "INFO"); err != nil {
		t.Fatalf("expected case-insensitive log.level to pass, got %v", err)
	}
	if err := validateValue("log.level", "  warn "); err != nil {
		t.Fatalf("expected trimmed log.level to pass, got %v", err)
	}
	if err := validateValue("log.level", 1); err == nil {
		t.Fatalf("expected non-string log.level to fail")
	}
	if err := validateValue("log.level", "trace"); err == nil {
		t.Fatalf("expected unsupported log.level to fail")
	}
}

func TestParsePathRejectsOverflowIndex(t *testing.T) {
	if _, err := parsePath("items[999999999999999999999999]"); err == nil {
		t.Fatalf("expected parsePath overflow index error")
	}
}

