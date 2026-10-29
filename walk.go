package sconfig

import (
	"encoding"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"time"
)

var (
	durationType        = reflect.TypeOf(time.Duration(0))
	jsonUnmarshalerType = reflect.TypeOf((*json.Unmarshaler)(nil)).Elem()
	textUnmarshalerType = reflect.TypeOf((*encoding.TextUnmarshaler)(nil)).Elem()
)

// walkStructs calls fn for every settable struct reachable from v: nested
// structs, non-nil pointers, slice and array elements and map values. Map
// values are copied, processed and written back. Unexported fields are
// skipped (except embedded structs); pointer cycles are visited once.
func walkStructs(v reflect.Value, fn func(reflect.Value) error) error {
	return walkValue(v, fn, map[uintptr]bool{})
}

func walkValue(v reflect.Value, fn func(reflect.Value) error, seen map[uintptr]bool) error {
	switch v.Kind() {
	case reflect.Ptr:
		if v.IsNil() || seen[v.Pointer()] {
			return nil
		}
		seen[v.Pointer()] = true
		return walkValue(v.Elem(), fn, seen)
	case reflect.Interface:
		if !v.IsNil() && v.Elem().Kind() == reflect.Ptr {
			return walkValue(v.Elem(), fn, seen)
		}
	case reflect.Struct:
		if err := fn(v); err != nil {
			return err
		}
		t := v.Type()
		for i := 0; i < t.NumField(); i++ {
			f := t.Field(i)
			if !f.IsExported() && !f.Anonymous {
				continue
			}
			if err := walkValue(v.Field(i), fn, seen); err != nil {
				return err
			}
		}
	case reflect.Slice, reflect.Array:
		if !mayContainStruct(v.Type().Elem()) {
			return nil
		}
		for i := 0; i < v.Len(); i++ {
			if err := walkValue(v.Index(i), fn, seen); err != nil {
				return err
			}
		}
	case reflect.Map:
		elemType := v.Type().Elem()
		if v.IsNil() || !mayContainStruct(elemType) {
			return nil
		}
		iter := v.MapRange()
		for iter.Next() {
			mv := iter.Value()
			if mv.Kind() == reflect.Ptr || mv.Kind() == reflect.Interface {
				// pointer targets are addressable, no write back needed
				if err := walkValue(mv, fn, seen); err != nil {
					return err
				}
				continue
			}
			cp := reflect.New(elemType).Elem()
			cp.Set(mv)
			if err := walkValue(cp, fn, seen); err != nil {
				return err
			}
			v.SetMapIndex(iter.Key(), cp)
		}
	}
	return nil
}

// mayContainStruct reports whether values of type t can contain structs.
func mayContainStruct(t reflect.Type) bool {
	switch t.Kind() {
	case reflect.Struct, reflect.Interface:
		return true
	case reflect.Ptr, reflect.Slice, reflect.Array, reflect.Map:
		return mayContainStruct(t.Elem())
	}
	return false
}

// applyDefaults sets the `default:"..."` values of all reachable structs.
func applyDefaults(v reflect.Value) error {
	return walkStructs(v, applyStructDefaults)
}

// applyStructDefaults sets the default tags of the direct fields of struct s.
func applyStructDefaults(s reflect.Value) error {
	st := s.Type()
	for i := 0; i < st.NumField(); i++ {
		field := st.Field(i)
		defaultValue, found := field.Tag.Lookup("default")
		if !found {
			continue
		}
		fieldValue := s.Field(i)
		if !fieldValue.CanSet() {
			return fmt.Errorf("%s", t("config.field_not_settable", field.Name))
		}
		if err := setDefaultValue(fieldValue, defaultValue); err != nil {
			return err
		}
	}
	return nil
}

// setDefaultValue parses defaultValue into the scalar field fv.
func setDefaultValue(fv reflect.Value, defaultValue string) error {
	if fv.Type() == durationType {
		d, err := time.ParseDuration(defaultValue)
		if err != nil {
			return fmt.Errorf(t("config.default_error"), err)
		}
		fv.SetInt(int64(d))
		return nil
	}
	switch fv.Kind() {
	case reflect.String:
		fv.SetString(defaultValue)
	case reflect.Bool:
		b, err := strconv.ParseBool(defaultValue)
		if err != nil {
			return fmt.Errorf(t("config.default_error"), err)
		}
		fv.SetBool(b)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		n, err := strconv.ParseInt(defaultValue, 10, fv.Type().Bits())
		if err != nil {
			return fmt.Errorf(t("config.default_error"), err)
		}
		fv.SetInt(n)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		n, err := strconv.ParseUint(defaultValue, 10, fv.Type().Bits())
		if err != nil {
			return fmt.Errorf(t("config.default_error"), err)
		}
		fv.SetUint(n)
	case reflect.Float32, reflect.Float64:
		f, err := strconv.ParseFloat(defaultValue, fv.Type().Bits())
		if err != nil {
			return fmt.Errorf(t("config.default_error"), err)
		}
		fv.SetFloat(f)
	default:
		return fmt.Errorf(t("config.default_unsupported"), fv.Kind())
	}
	return nil
}

// typeHasDefaults reports whether type t (or anything it contains) has
// default tags. seen guards against recursive types.
func typeHasDefaults(t reflect.Type, seen map[reflect.Type]bool) bool {
	if done, ok := seen[t]; ok {
		return done
	}
	seen[t] = false
	result := false
	switch t.Kind() {
	case reflect.Struct:
		for i := 0; i < t.NumField() && !result; i++ {
			f := t.Field(i)
			if !f.IsExported() && !f.Anonymous {
				continue
			}
			if _, ok := f.Tag.Lookup("default"); ok || typeHasDefaults(f.Type, seen) {
				result = true
			}
		}
	case reflect.Ptr, reflect.Slice, reflect.Array, reflect.Map:
		result = typeHasDefaults(t.Elem(), seen)
	}
	seen[t] = result
	return result
}

// hasCustomUnmarshal reports whether t decodes itself (json.Unmarshaler or
// encoding.TextUnmarshaler); such values are left untouched.
func hasCustomUnmarshal(t reflect.Type) bool {
	pt := reflect.PointerTo(t)
	return t.Implements(jsonUnmarshalerType) || pt.Implements(jsonUnmarshalerType) ||
		t.Implements(textUnmarshalerType) || pt.Implements(textUnmarshalerType)
}

// jsonFieldName returns the JSON key of a struct field like encoding/json
// does; skip is true for `json:"-"`.
func jsonFieldName(f reflect.StructField) (name string, skip bool) {
	tag := f.Tag.Get("json")
	if tag == "-" {
		return "", true
	}
	name, _, _ = strings.Cut(tag, ",")
	if name == "" {
		name = f.Name
	}
	return name, false
}

// lookupJSONKey finds key in obj: exact match first, then case-insensitive
// (encoding/json matches keys case-insensitively).
func lookupJSONKey(obj map[string]json.RawMessage, key string) (json.RawMessage, bool) {
	if raw, ok := obj[key]; ok {
		return raw, true
	}
	for k, raw := range obj {
		if strings.EqualFold(k, key) {
			return raw, true
		}
	}
	return nil, false
}

// applyDecodedDefaults gives values that encoding/json created from the file
// (slice/array/map elements and pointer targets) their default tags: each
// such element is rebuilt as "defaults, then the element's JSON". Struct
// fields that already existed before decoding got their defaults from
// applyDefaults and are only descended into.
func applyDecodedDefaults(s reflect.Value, raw json.RawMessage) error {
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(raw, &obj); err != nil || obj == nil {
		return nil // not an object: nothing was decoded into the fields
	}
	t := s.Type()
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		fv := s.Field(i)
		if !f.IsExported() && !f.Anonymous {
			continue
		}
		if f.Anonymous && f.Tag.Get("json") == "" {
			// promoted fields of an embedded struct share the object
			if fv.Kind() == reflect.Ptr && !fv.IsNil() {
				fv = fv.Elem()
			}
			if fv.Kind() == reflect.Struct {
				if err := applyDecodedDefaults(fv, raw); err != nil {
					return err
				}
			}
			continue
		}
		name, skip := jsonFieldName(f)
		if skip || !fv.CanSet() {
			continue
		}
		fieldRaw, ok := lookupJSONKey(obj, name)
		if !ok || string(fieldRaw) == "null" {
			continue
		}
		if err := applyDecodedDefaultsValue(fv, fieldRaw); err != nil {
			return err
		}
	}
	return nil
}

// applyDecodedDefaultsValue handles one decoded value according to its kind.
func applyDecodedDefaultsValue(fv reflect.Value, raw json.RawMessage) error {
	ft := fv.Type()
	if !typeHasDefaults(ft, map[reflect.Type]bool{}) || hasCustomUnmarshal(ft) {
		return nil
	}
	switch ft.Kind() {
	case reflect.Struct:
		return applyDecodedDefaults(fv, raw)
	case reflect.Ptr:
		if fv.IsNil() {
			return nil
		}
		rebuilt, err := rebuildWithDefaults(ft, raw)
		if err != nil {
			return err
		}
		fv.Set(rebuilt)
	case reflect.Slice, reflect.Array:
		var items []json.RawMessage
		if err := json.Unmarshal(raw, &items); err != nil {
			return nil
		}
		for i := 0; i < fv.Len() && i < len(items); i++ {
			rebuilt, err := rebuildWithDefaults(ft.Elem(), items[i])
			if err != nil {
				return err
			}
			fv.Index(i).Set(rebuilt)
		}
	case reflect.Map:
		if ft.Key().Kind() != reflect.String || fv.IsNil() {
			return nil
		}
		var items map[string]json.RawMessage
		if err := json.Unmarshal(raw, &items); err != nil {
			return nil
		}
		for k, itemRaw := range items {
			key := reflect.ValueOf(k).Convert(ft.Key())
			if !fv.MapIndex(key).IsValid() {
				continue
			}
			rebuilt, err := rebuildWithDefaults(ft.Elem(), itemRaw)
			if err != nil {
				return err
			}
			fv.SetMapIndex(key, rebuilt)
		}
	}
	return nil
}

// rebuildWithDefaults creates a new value of type t, applies the default tags
// and then decodes raw into it.
func rebuildWithDefaults(t reflect.Type, raw json.RawMessage) (reflect.Value, error) {
	if t.Kind() == reflect.Ptr {
		if string(raw) == "null" {
			return reflect.Zero(t), nil
		}
		elem, err := rebuildWithDefaults(t.Elem(), raw)
		if err != nil {
			return reflect.Value{}, err
		}
		p := reflect.New(t.Elem())
		p.Elem().Set(elem)
		return p, nil
	}
	n := reflect.New(t).Elem()
	if !hasCustomUnmarshal(t) {
		if err := applyDefaults(n); err != nil {
			return reflect.Value{}, err
		}
	}
	if err := json.Unmarshal(raw, n.Addr().Interface()); err != nil {
		return reflect.Value{}, err
	}
	if n.Kind() == reflect.Struct {
		if err := applyDecodedDefaults(n, raw); err != nil {
			return reflect.Value{}, err
		}
	} else if err := applyDecodedDefaultsValue(n, raw); err != nil {
		return reflect.Value{}, err
	}
	return n, nil
}

// findPasswordField returns the index of field <prefix>Password in struct
// type t (same struct level only), or -1.
func findPasswordField(t reflect.Type, prefix string) int {
	for j := 0; j < t.NumField(); j++ {
		if t.Field(j).Name == prefix+"Password" {
			return j
		}
	}
	return -1
}

// forEachPasswordPair calls fn for every <Name>SecurePassword/<Name>Password
// string field pair of struct s.
func forEachPasswordPair(s reflect.Value, fn func(prefix string, secure, plain reflect.Value) error) error {
	st := s.Type()
	for i := 0; i < st.NumField(); i++ {
		f := st.Field(i)
		if !f.IsExported() || !strings.HasSuffix(f.Name, "SecurePassword") || f.Type.Kind() != reflect.String {
			continue
		}
		prefix := strings.TrimSuffix(f.Name, "SecurePassword")
		j := findPasswordField(st, prefix)
		if j < 0 || st.Field(j).Type.Kind() != reflect.String {
			continue
		}
		secure, plain := s.Field(i), s.Field(j)
		if !secure.CanSet() || !plain.CanSet() {
			return fmt.Errorf("%s", t("config.field_not_settable", prefix+"Password"))
		}
		if err := fn(prefix, secure, plain); err != nil {
			return err
		}
	}
	return nil
}

// migrationStats counts the legacy values handled by one encryptPasswords
// call (for the warnings after a load).
type migrationStats struct {
	migrated int
	failed   int
}

// encryptPasswords encrypts new plaintext passwords (v2), migrates legacy v1
// values to v2 and re-encrypts v2 values that use fewer sources than are
// available now. *changed is set when the file must be written. Warnings are
// issued for migrated values (rotate passwords) and failed migrations.
func encryptPasswords(v reflect.Value, changed *bool) error {
	var stats migrationStats
	err := walkStructs(v, func(s reflect.Value) error {
		return forEachPasswordPair(s, func(prefix string, secure, plain reflect.Value) error {
			passwordVal, secureVal := plain.String(), secure.String()
			if isPasswordMarker(passwordVal) {
				if secureVal == "" {
					return passwordPairError(prefix, "config.secure_password_missing")
				}
				if !isV2Value(secureVal) {
					return reencryptValue(prefix, secure, changed, &stats, true)
				}
				if needsUpgrade(secureVal) {
					return reencryptValue(prefix, secure, changed, &stats, false)
				}
				return nil
			}
			if passwordVal == "" {
				return nil
			}
			encrypted, err := encrypt(passwordVal, prefix)
			if err != nil {
				return err
			}
			secure.SetString(encrypted)
			plain.SetString(PASSWORD_IS_SECURE)
			*changed = true
			return nil
		})
	})
	if err != nil {
		return err
	}
	if stats.migrated > 0 {
		warn(t("config.warn_migrated", stats.migrated))
	}
	if stats.failed > 0 {
		warn(t("config.warn_migration_failed", stats.failed))
	}
	return nil
}

// reencryptValue decrypts a value and encrypts it again with all current v2
// sources. If that is not possible, the readable old value is kept; for
// legacy values this is counted as failed migration.
func reencryptValue(prefix string, secure reflect.Value, changed *bool, stats *migrationStats, legacy bool) error {
	plain, err := decrypt(secure.String(), prefix)
	if err != nil {
		return decryptError(prefix, err)
	}
	encrypted, err := encrypt(plain, prefix)
	if err != nil {
		if legacy {
			stats.failed++
		}
		return nil
	}
	secure.SetString(encrypted)
	*changed = true
	if legacy {
		stats.migrated++
	}
	return nil
}

// isInvalidCiphertext reports whether err marks a structurally invalid value.
func isInvalidCiphertext(err error) bool {
	return errors.Is(err, errInvalidCiphertext)
}

// decryptError maps a decrypt error to the matching user message.
func decryptError(prefix string, err error) error {
	if debugMode {
		writeDebugLog(lastDebugHardwareID, lastDebugIdentifiers, false)
	}
	if isInvalidCiphertext(err) {
		return passwordPairError(prefix, "config.encrypted_password_invalid", err)
	}
	return passwordPairError(prefix, "config.decrypt_failed", err)
}

// decodePasswords decrypts all secure password fields into their plaintext
// fields so that the encryption is transparent for the application.
func decodePasswords(v reflect.Value) error {
	return walkStructs(v, func(s reflect.Value) error {
		return forEachPasswordPair(s, func(prefix string, secure, plain reflect.Value) error {
			secureVal := secure.String()
			if secureVal == "" {
				if isPasswordMarker(plain.String()) {
					return passwordPairError(prefix, "config.secure_password_missing")
				}
				return nil
			}
			password, err := decrypt(secureVal, prefix)
			if err != nil {
				return decryptError(prefix, err)
			}
			plain.SetString(password)
			return nil
		})
	})
}
