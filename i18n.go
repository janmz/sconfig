package sconfig

import (
	"embed"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/nicksnyder/go-i18n/v2/i18n"
	"golang.org/x/text/language"
)

//go:embed locales/*.json
var localesFS embed.FS

// passwordMessageKey is the translation key of the password marker. Its text
// is always taken from the embedded translations so that external locale
// files cannot change the marker (a changed marker would turn the old marker
// into a "new plaintext password").
const passwordMessageKey = "config.password_message"

var (
	i18nMu sync.RWMutex
	// bundle holds the embedded and the external translations.
	bundle *i18n.Bundle
	// embeddedBundle holds only the embedded translations (markers).
	embeddedBundle *i18n.Bundle
	localizer      *i18n.Localizer
	currentLang    = "en"
)

// init loads the embedded translations and selects the system language.
func init() {
	bundle = newEmbeddedBundle()
	embeddedBundle = newEmbeddedBundle()
	setLanguage(detectLanguage())
}

// newEmbeddedBundle creates a bundle with English as fallback and the
// embedded en/de translations.
func newEmbeddedBundle() *i18n.Bundle {
	b := i18n.NewBundle(language.English)
	b.RegisterUnmarshalFunc("json", json.Unmarshal)
	for _, name := range []string{"en.json", "de.json"} {
		if data, err := localesFS.ReadFile("locales/" + name); err == nil {
			// The embedded files are validated by the tests; an error only
			// means that untranslated keys are returned as they are.
			_, _ = b.ParseMessageFileBytes(data, name)
		}
	}
	return b
}

// detectLanguage determines the language from the environment using the
// POSIX precedence LC_ALL > LC_MESSAGES > LANG.
func detectLanguage() string {
	for _, env := range []string{"LC_ALL", "LC_MESSAGES", "LANG"} {
		if lang := os.Getenv(env); lang != "" {
			if strings.HasPrefix(strings.ToLower(lang), "de") {
				return "de"
			}
			return "en"
		}
	}
	return "en"
}

// SetLocalesDir loads additional or overriding translations from all *.json
// files in dir (file name = language, e.g. "fr.json", format like
// locales/en.json). All files are validated first; on error nothing is
// changed. The password marker always stays the embedded text.
func SetLocalesDir(dir string) error {
	files, err := filepath.Glob(filepath.Join(dir, "*.json"))
	if err != nil {
		return err
	}
	contents := make(map[string][]byte, len(files))
	check := i18n.NewBundle(language.English)
	check.RegisterUnmarshalFunc("json", json.Unmarshal)
	for _, file := range files {
		data, err := readFileLimited(file, maxLocaleFileSize)
		if err != nil {
			return err
		}
		if _, err := check.ParseMessageFileBytes(data, filepath.Base(file)); err != nil {
			return fmt.Errorf("%s: %w", file, err)
		}
		contents[file] = data
	}
	i18nMu.Lock()
	defer i18nMu.Unlock()
	for file, data := range contents {
		if _, err := bundle.ParseMessageFileBytes(data, filepath.Base(file)); err != nil {
			return fmt.Errorf("%s: %w", file, err)
		}
	}
	localizer = i18n.NewLocalizer(bundle, currentLang)
	return nil
}

// setLanguage sets the current language.
func setLanguage(lang string) {
	i18nMu.Lock()
	defer i18nMu.Unlock()
	currentLang = lang
	localizer = i18n.NewLocalizer(bundle, lang)
}

// getCurrentLanguage returns the current language code.
func getCurrentLanguage() string {
	i18nMu.RLock()
	defer i18nMu.RUnlock()
	return currentLang
}

// markerText returns the password marker of a language from the embedded
// translations only.
func markerText(lang string) string {
	msg, err := i18n.NewLocalizer(embeddedBundle, lang).Localize(&i18n.LocalizeConfig{MessageID: passwordMessageKey})
	if err != nil {
		return "Enter new password here"
	}
	return msg
}

// translate translates a key to the current language and formats it with
// fmt.Sprintf-style arguments.
func translate(key string, args ...interface{}) string {
	i18nMu.RLock()
	loc := localizer
	b := bundle
	i18nMu.RUnlock()

	if loc == nil {
		if len(args) > 0 {
			return fmt.Sprintf(key+": %v", args)
		}
		return key
	}

	// Template data keeps the go-i18n call happy; the message itself uses
	// fmt verbs and is formatted below.
	templateData := make(map[string]interface{})
	if len(args) == 1 {
		templateData["arg"] = args[0]
	} else {
		for i := range args {
			templateData[fmt.Sprintf("arg%d", i)] = args[i]
		}
	}

	cfg := &i18n.LocalizeConfig{MessageID: key, TemplateData: templateData}
	msg, err := loc.Localize(cfg)
	if err != nil {
		msg, err = i18n.NewLocalizer(b, "en").Localize(cfg)
		if err != nil {
			if len(args) > 0 {
				return fmt.Sprintf(key+": %v", args)
			}
			return key
		}
	}
	if len(args) > 0 {
		return fmt.Sprintf(msg, args...)
	}
	return msg
}

// t is the short form of translate.
func t(key string, args ...interface{}) string {
	return translate(key, args...)
}
