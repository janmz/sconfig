package sconfig

import (
	"embed"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/nicksnyder/go-i18n/v2/i18n"
	"golang.org/x/text/language"
)

//go:embed locales/*.json
var localesFS embed.FS

var (
	bundle      *i18n.Bundle
	localizer   *i18n.Localizer
	currentLang = "en"
)

// Initialize i18n system
func init() {
	// Create bundle with English as fallback
	bundle = i18n.NewBundle(language.English)
	bundle.RegisterUnmarshalFunc("json", json.Unmarshal)

	// Detect system language
	lang := detectLanguage()

	// Load translations
	if err := loadTranslations(); err != nil {
		panic(fmt.Errorf("failed to load translations: %v", err))
	}

	setLanguage(lang)
}

// detectLanguage tries to detect system language
func detectLanguage() string {
	// Try environment variables
	for _, env := range []string{"LANG", "LC_ALL", "LC_MESSAGES"} {
		if lang := os.Getenv(env); lang != "" {
			if strings.HasPrefix(strings.ToLower(lang), "de") {
				return "de"
			}
			return "en"
		}
	}

	// Default to English
	return "en"
}

// loadTranslations loads translation files from embedded data or external files
func loadTranslations() error {
	// Load embedded translations first
	if err := loadEmbeddedTranslations(); err != nil {
		return err
	}

	// Try to load external translation files (these will override embedded ones)
	loadExternalTranslations()

	return nil
}

// loadEmbeddedTranslations loads translations from embedded files
func loadEmbeddedTranslations() error {
	// Load English translations
	enData, err := localesFS.ReadFile("locales/en.json")
	if err == nil {
		bundle.MustParseMessageFileBytes(enData, "en.json")
	}

	// Load German translations
	deData, err := localesFS.ReadFile("locales/de.json")
	if err == nil {
		bundle.MustParseMessageFileBytes(deData, "de.json")
	}

	return nil
}

// loadExternalTranslations tries to load translation files from locales directory
func loadExternalTranslations() {
	localesDir := "locales"
	if _, err := os.Stat(localesDir); os.IsNotExist(err) {
		return
	}

	files, err := filepath.Glob(filepath.Join(localesDir, "*.json"))
	if err != nil {
		return
	}

	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			continue
		}

		// Parse and add to bundle
		bundle.MustParseMessageFileBytes(data, filepath.Base(file))
	}
}

// setLanguage sets the current language
func setLanguage(lang string) {
	currentLang = lang
	// Create localizer for the current language
	localizer = i18n.NewLocalizer(bundle, lang)
}

// translate translates a key to the current language
func translate(key string, args ...interface{}) string {
	if localizer == nil {
		// Fallback if localizer is not initialized
		if len(args) > 0 {
			return fmt.Sprintf(key+": %v", args)
		}
		return key
	}

	// Convert args to template data if needed
	templateData := make(map[string]interface{})
	if len(args) > 0 {
		// For simple cases, we'll use the first arg as a string
		// This maintains compatibility with the existing t() function calls
		if len(args) == 1 {
			templateData["arg"] = args[0]
		} else {
			// For multiple args, we'll format them as before
			for i := range args {
				templateData[fmt.Sprintf("arg%d", i)] = args[i]
			}
		}
	}

	// Try to localize the message
	msg, err := localizer.Localize(&i18n.LocalizeConfig{
		MessageID:    key,
		TemplateData: templateData,
	})

	if err != nil {
		// If localization fails, try fallback
		fallbackLocalizer := i18n.NewLocalizer(bundle, "en")
		msg, err = fallbackLocalizer.Localize(&i18n.LocalizeConfig{
			MessageID:    key,
			TemplateData: templateData,
		})

		if err != nil {
			// If still no translation found, return key with args
			if len(args) > 0 {
				return fmt.Sprintf(key+": %v", args)
			}
			return key
		}
	}

	// If we have template data, format the message
	if len(templateData) > 0 {
		return fmt.Sprintf(msg, args...)
	}

	return msg
}

// Helper functions for easy access
func t(key string, args ...interface{}) string {
	return translate(key, args...)
}

// getCurrentLanguage returns the current language code
func getCurrentLanguage() string {
	return currentLang
}
