//go:build ignore

package main

import (
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

type I18nChecker struct {
	translationsDir string
	sourceDir       string
	languages       []string

	// Results
	usedKeys    map[string]bool
	definedKeys map[string]map[string]bool
	missingKeys map[string][]string
	unusedKeys  map[string][]string
}

func main() {
	fmt.Printf("=== Checking Environment ===\n")
	if _, err := os.Stat(".i18n-config"); err == nil {
		fmt.Printf("Reading .i18n-config\n")
		file, err := os.ReadFile(".i18n-config")
		if err != nil {
			fmt.Fprintf(os.Stderr, "failed to read .i18n-config: %v\n", err)
			os.Exit(1)
		}
		lines := strings.Split(string(file), "\n")
		for _, line := range lines {
			if strings.TrimSpace(line) == "" || strings.HasPrefix(line, "#") {
				continue // Skip empty lines and comments
			}
			parts := strings.SplitN(line, "=", 2)
			if len(parts) == 2 {
				key := strings.TrimSpace(parts[0])
				value := strings.TrimSpace(parts[1])
				os.Setenv(key, value) // Set the environment variable
			}
		}
	}
	// Configuration from environment variables or default values
	translationsDir := getEnvOrDefault("I18N_TRANSLATIONS_DIR", "./locales")
	sourceDir := getEnvOrDefault("I18N_SOURCE_DIR", ".")
	languagesStr := getEnvOrDefault("I18N_LANGUAGES", "en,de,fr")

	languages := strings.Split(languagesStr, ",")
	for i, lang := range languages {
		languages[i] = strings.TrimSpace(lang)
	}

	checker := &I18nChecker{
		translationsDir: translationsDir,
		sourceDir:       sourceDir,
		languages:       languages,
		usedKeys:        make(map[string]bool),
		definedKeys:     make(map[string]map[string]bool),
		missingKeys:     make(map[string][]string),
		unusedKeys:      make(map[string][]string),
	}

	fmt.Printf("=== i18n Keys Validation ===\n")
	fmt.Printf("Source directory: %s\n", checker.sourceDir)
	fmt.Printf("Translations directory: %s\n", checker.translationsDir)
	fmt.Printf("Languages: %v\n", checker.languages)
	fmt.Println()

	if err := checker.run(); err != nil {
		fmt.Printf("❌ Error: %v\n", err)
		os.Exit(1)
	}
}

func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func (c *I18nChecker) run() error {
	// 1. Find all used i18n keys in source code
	fmt.Println("📝 Scanning source code for i18n keys...")
	if err := c.findUsedKeys(); err != nil {
		return fmt.Errorf("finding used keys: %w", err)
	}
	fmt.Printf("   Found %d unique i18n keys\n", len(c.usedKeys))

	// 2. Load all translation files
	fmt.Println("📂 Loading translation files...")
	if err := c.loadTranslations(); err != nil {
		return fmt.Errorf("loading translations: %w", err)
	}

	// 3. Analyze and report
	c.analyze()
	exitCode := c.report()

	if exitCode != 0 {
		os.Exit(exitCode)
	}

	return nil
}

// findUsedKeys scans Go source files for i18n key usage
func (c *I18nChecker) findUsedKeys() error {
	return filepath.WalkDir(c.sourceDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		// Skip vendor, .git, node_modules, etc.
		if d.IsDir() {
			name := d.Name()
			if name == "vendor" || name == ".git" || name == "node_modules" ||
				(strings.HasPrefix(name, ".") && len(name) > 1) || name == "locales" {
				return filepath.SkipDir
			}
			return nil
		} else {

			// Only process .go files (skip test files)
			if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}

			return c.scanGoFile(path)
		}
	})
}

// scanGoFile parses a Go file and extracts i18n keys
func (c *I18nChecker) scanGoFile(filename string) error {
	fmt.Printf("Checking file %s...\n", filename)
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, filename, nil, parser.ParseComments)
	if err != nil {
		// Skip files with parse errors
		return nil
	}

	ast.Inspect(node, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}

		// Look for i18n function calls
		if c.isI18nCall(call) {
			if key := c.extractKeyFromCall(call); key != "" {
				c.usedKeys[key] = true
			}
		}

		return true
	})

	return nil
}

// isI18nCall checks if a function call is an i18n function
func (c *I18nChecker) isI18nCall(call *ast.CallExpr) bool {
	switch fun := call.Fun.(type) {
	case *ast.Ident:
		// Direct function calls like T(), Localize(), _()
		return fun.Name == "t" || fun.Name == "T" || fun.Name == "Localize" || fun.Name == "MustLocalize" || fun.Name == "_"

	case *ast.SelectorExpr:
		// Method calls like localizer.Localize(), i18n.T(), ctx.Tr()
		return fun.Sel.Name == "Localize" || fun.Sel.Name == "MustLocalize" ||
			fun.Sel.Name == "t" || fun.Sel.Name == "T" || fun.Sel.Name == "Tr" || fun.Sel.Name == "_"
	}

	return false
}

// extractKeyFromCall extracts the translation key from a function call
func (c *I18nChecker) extractKeyFromCall(call *ast.CallExpr) string {
	if len(call.Args) == 0 {
		return ""
	}

	// Handle different call patterns
	switch arg := call.Args[0].(type) {
	case *ast.BasicLit:
		// Direct string: T("key")
		if arg.Kind == token.STRING {
			return strings.Trim(arg.Value, `"`)
		}

	case *ast.CompositeLit:
		// Struct literal: Localize(&i18n.LocalizeConfig{MessageID: "key"})
		for _, elt := range arg.Elts {
			if kv, ok := elt.(*ast.KeyValueExpr); ok {
				if ident, ok := kv.Key.(*ast.Ident); ok &&
					(ident.Name == "MessageID" || ident.Name == "ID") {
					if lit, ok := kv.Value.(*ast.BasicLit); ok && lit.Kind == token.STRING {
						return strings.Trim(lit.Value, `"`)
					}
				}
			}
		}
	}

	return ""
}

// Initialize definedKeys for all languages
func (c *I18nChecker) initializeDefinedKeys() {
	for _, lang := range c.languages {
		c.definedKeys[lang] = make(map[string]bool)
	}
}

// loadTranslations loads all translation files
func (c *I18nChecker) loadTranslations() error {
	c.initializeDefinedKeys()

	for _, lang := range c.languages {
		// Try different file formats and naming patterns
		files := []string{
			filepath.Join(c.translationsDir, fmt.Sprintf("active.%s.json", lang)),
			filepath.Join(c.translationsDir, fmt.Sprintf("active.%s.toml", lang)),
			filepath.Join(c.translationsDir, fmt.Sprintf("%s.json", lang)),
			filepath.Join(c.translationsDir, fmt.Sprintf("%s.toml", lang)),
			filepath.Join(c.translationsDir, lang, "messages.json"),
			filepath.Join(c.translationsDir, lang, "translation.json"),
		}

		var loaded bool
		for _, file := range files {
			if err := c.loadTranslationFile(file, lang); err == nil {
				fmt.Printf("   Loaded %s (%d keys)\n", file, len(c.definedKeys[lang]))
				loaded = true
				break
			}
		}

		if !loaded {
			fmt.Printf("   ⚠️  No translation file found for language: %s\n", lang)
		}
	}

	return nil
}

// loadTranslationFile loads a single translation file
func (c *I18nChecker) loadTranslationFile(filename, lang string) error {
	data, err := os.ReadFile(filename)
	if err != nil {
		return err
	}

	if strings.HasSuffix(filename, ".json") {
		return c.loadJSONTranslations(data, lang)
	} else if strings.HasSuffix(filename, ".toml") {
		return c.loadTOMLTranslations(data, lang)
	}

	return fmt.Errorf("unsupported file format: %s", filename)
}

// loadJSONTranslations loads JSON format translations
func (c *I18nChecker) loadJSONTranslations(data []byte, lang string) error {
	var translations map[string]interface{}
	if err := json.Unmarshal(data, &translations); err != nil {
		return err
	}

	c.extractKeysFromMap(translations, "", lang)
	return nil
}

// loadTOMLTranslations loads TOML format (simplified, for go-i18n format)
func (c *I18nChecker) loadTOMLTranslations(data []byte, lang string) error {
	// Simple regex-based parsing for go-i18n TOML format
	content := string(data)

	// Match message blocks: [message_id] or [[message]] with id = "..."
	blockRe := regexp.MustCompile(`\[([^\]]+)\]`)
	idRe := regexp.MustCompile(`id\s*=\s*"([^"]+)"`)

	// First try block-style [message_id]
	blocks := blockRe.FindAllStringSubmatch(content, -1)
	for _, block := range blocks {
		if len(block) > 1 {
			key := block[1]
			if key != "" && !strings.Contains(key, "[") { // Skip [[array]] style
				c.definedKeys[lang][key] = true
			}
		}
	}

	// Then try id = "..." style
	ids := idRe.FindAllStringSubmatch(content, -1)
	for _, id := range ids {
		if len(id) > 1 {
			key := id[1]
			if key != "" {
				c.definedKeys[lang][key] = true
			}
		}
	}

	return nil
}

// extractKeysFromMap recursively extracts keys from nested map
func (c *I18nChecker) extractKeysFromMap(m map[string]interface{}, prefix, lang string) {
	for key, value := range m {
		fullKey := key
		if prefix != "" {
			fullKey = prefix + "." + key
		}

		switch v := value.(type) {
		case map[string]interface{}:
			c.extractKeysFromMap(v, fullKey, lang)
		default:
			// Any non-map value is considered a translation
			c.definedKeys[lang][fullKey] = true
		}
	}
}

// analyze compares used vs defined keys
func (c *I18nChecker) analyze() {
	fmt.Println("🔍 Analyzing translations...")

	for _, lang := range c.languages {
		defined := c.definedKeys[lang]

		// Find missing keys (used but not defined)
		for usedKey := range c.usedKeys {
			if !defined[usedKey] {
				c.missingKeys[lang] = append(c.missingKeys[lang], usedKey)
			}
		}

		// Find unused keys (defined but not used)
		for definedKey := range defined {
			if !c.usedKeys[definedKey] {
				c.unusedKeys[lang] = append(c.unusedKeys[lang], definedKey)
			}
		}

		// Sort for consistent output
		sort.Strings(c.missingKeys[lang])
		sort.Strings(c.unusedKeys[lang])
	}
}

// report prints the analysis results
func (c *I18nChecker) report() int {
	fmt.Printf("\n=== Analysis Results ===\n")
	fmt.Printf("Found %d used i18n keys in source code\n\n", len(c.usedKeys))

	hasErrors := false
	hasWarnings := false

	// Report per language
	for _, lang := range c.languages {
		missing := c.missingKeys[lang]
		unused := c.unusedKeys[lang]
		total := len(c.definedKeys[lang])

		fmt.Printf("Language: %s (%d defined keys)\n", lang, total)

		if len(missing) > 0 {
			fmt.Printf("  ❌ Missing %d translations:\n", len(missing))
			for _, key := range missing {
				fmt.Printf("     - %s\n", key)
			}
			hasErrors = true
		} else {
			fmt.Printf("  ✅ All translations present\n")
		}

		if len(unused) > 0 {
			fmt.Printf("  ⚠️  Unused %d translations:\n", len(unused))
			for _, key := range unused[:min(10, len(unused))] { // Show max 10
				fmt.Printf("     - %s\n", key)
			}
			if len(unused) > 10 {
				fmt.Printf("     ... and %d more\n", len(unused)-10)
			}
			hasWarnings = true
		}

		fmt.Println()
	}

	// Print used keys if verbose mode
	if os.Getenv("I18N_VERBOSE") == "true" {
		fmt.Println("=== Used Keys ===")
		var keys []string
		for key := range c.usedKeys {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			fmt.Printf("  - %s\n", key)
		}
		fmt.Println()
	}

	// Summary
	if hasErrors {
		fmt.Println("❌ i18n validation failed - missing translations found")
		return 1
	} else if hasWarnings {
		fmt.Println("⚠️  i18n validation completed with warnings - unused translations found")
		if os.Getenv("I18N_STRICT") == "true" {
			return 1
		}
		return 0
	} else {
		fmt.Println("✅ i18n validation successful")
		return 0
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
