package sconfig

import (
	"fmt"
	"testing"
)

func TestI18n(ts *testing.T) {
	// Test the t() function with different keys
	ts.Run("Testing i18n functionality", func(ts *testing.T) {
		currLang := getCurrentLanguage()
		if getCurrentLanguage() == "de" {
			setLanguage("en")
			currLang = getCurrentLanguage()
			if getCurrentLanguage() != "en" {
				ts.Errorf("setLanguage('en') resulted in getCurrentLanguage()='%v'", currLang)
			}
		} else if getCurrentLanguage() != "en" {
			ts.Errorf("getCurrentLanguage should return 'de' or 'en' but returned '%v'", currLang)
			setLanguage("en")
			currLang = getCurrentLanguage()
			if getCurrentLanguage() != "en" {
				ts.Errorf("setLanguage('en') resulted in getCurrentLanguage()='%v'", currLang)
			}
		} else {
			setLanguage("de")
			currLang = getCurrentLanguage()
			if getCurrentLanguage() != "de" {
				ts.Errorf("setLanguage('de') resulted in getCurrentLanguage()='%v'", currLang)
			}
			setLanguage("en")
			currLang = getCurrentLanguage()
			if getCurrentLanguage() != "en" {
				ts.Errorf("setLanguage('en') resulted in getCurrentLanguage()='%v'", currLang)
			}
		}
	})

	// Test some translations
	results := [][]string{
		{"test.app.name", "Testing i18n for sconfig", "Teste i18n für sconfig"},
		{"test.app.version", "Version 1.0.0 from 2024-01-01", "Version 1.0.0 vom 2024-01-01", "1.0.0", "2024-01-01"},
		{"test.app.working_directory", "Working directory is '/path/to/dir'", "Das Arbeitsverzeichnis ist '/path/to/dir'", "/path/to/dir"},
		{"test.error.no_directory", "Folder '/invalid/path' does not exist.", "Das Verzeichnis '/invalid/path' wurde nicht gefunden.", "/invalid/path"},
	}
	ts.Run("Testing translations to English", func(ts *testing.T) {
		for _, entry := range results {
			result := ""
			if len(entry) >= 6 {
				result = t(entry[0], entry[3], entry[4], entry[5])
			} else if len(entry) == 5 {
				result = t(entry[0], entry[3], entry[4])
			} else if len(entry) == 4 {
				result = t(entry[0], entry[3])
			} else {
				result = t(entry[0])
			}
			if entry[1] != result {
				ts.Errorf("Translation for key %s should be %s but was '%s'", entry[0], entry[1], result)
			}
		}
	})
	// Test German language
	setLanguage("de")
	ts.Run("Testing translations to German", func(ts *testing.T) {
		for _, entry := range results {
			result := ""
			if len(entry) >= 6 {
				result = t(entry[0], entry[3], entry[4], entry[5])
			} else if len(entry) == 5 {
				result = t(entry[0], entry[3], entry[4])
			} else if len(entry) == 4 {
				result = t(entry[0], entry[3])
			} else {
				result = t(entry[0])
			}
			if entry[2] != result {
				ts.Errorf("Translation for key %s should be %s but was '%s'", entry[0], entry[2], result)
			}
		}
	})
	ts.Run("Testing fallback for unknown key", func(ts *testing.T) {
		expected := fmt.Sprintf("unknown.key: %v", []string{"test arg"})
		result := t("unknown.key", "test arg")
		if result != expected {
			ts.Errorf("Translation for unknown key 'unknown.key' should be %s but was '%s'", expected, result)
		}
	})
}
