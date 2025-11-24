package sconfig

/*
 * Description: This package contains a function for managing config files with secure passwords.
 *
 * Version 1.0.0

 * Changelog:
 * 1.2.0  24.11.25 Included PHP variant
 *
 * Author: Jan Neuhaus, VAYA Consulting, https://vaya-consultig.de/development/ https://github.com/janmz
 *
 * Functions:
 * - LoadConfig(): Loads the configuration from a file and processes it, it may rewrite it to encode passwords.
 *
 * Dependencies:
 * - i18n.go and locales/*.json: For internationalization of error messages
 */

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	mathRand "math/rand"
	"net"
	"os"
	"os/exec"
	"reflect"
	"runtime"
	"strconv"
	"strings"

	"crypto/aes"    // AES Encryption
	"crypto/cipher" // Cipher for GCM
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64" // Base64 Encoding
)

// PASSWORD_IS_SECURE is the marker written to plaintext password fields after
// successful encryption. Any other value in a `<Name>Password` field is treated
// as a new plaintext password and will be encrypted and replaced by this marker.
var PASSWORD_IS_SECURE string

// PASSWORD_IS_SECURE_en is the English variant of the marker that is recognized
// when deciding whether a password field already contains an encrypted value.
var PASSWORD_IS_SECURE_en string

// PASSWORD_IS_SECURE_de is the German variant of the marker that is recognized
// when deciding whether a password field already contains an encrypted value.
var PASSWORD_IS_SECURE_de string

var encryptionKey []byte
var initialized = false

/*
 * This function is a default function that can be overridden and generates a 64-bit number
 * that uniquely identifies a system in such a way that it is unlikely that someone can simply build
 * a second system that gets the identical ID. This allows system-specific keys to be generated
 * that are used for encrypting passwords in config files.
 *
 */
func secure_config_getHardwareID() (uint64, error) {
	var identifiers []string

	// MAC address of the first network card
	interfaces, err := net.Interfaces()
	if err == nil && len(interfaces) > 0 {
		for _, iface := range interfaces {
			if iface.HardwareAddr != nil {
				identifiers = append(identifiers, iface.HardwareAddr.String())
				break
			}
		}
	}

	// CPU ID and other hardware information depending on the operating system
	switch runtime.GOOS {
	case "windows":
		// Windows-specific hardware IDs
		cmds := []string{
			"wmic cpu get ProcessorId",
			"wmic baseboard get SerialNumber",
			"wmic baseboard get Product",
			"wmic diskdrive get SerialNumber",
		}

		for _, cmd := range cmds {
			out, err := exec.Command("cmd", "/C", cmd).Output()
			if err == nil {
				lines := strings.Split(string(out), "\n")
				if len(lines) > 1 {
					// First line is the header, second line contains the value
					value := strings.TrimSpace(lines[1])
					if value != "" {
						identifiers = append(identifiers, value)
					}
				}
			}
		}

	case "linux":
		// Linux-specific hardware IDs
		cmds := []string{
			"cat /proc/cpuinfo | grep 'Serial'",
			"cat /sys/class/dmi/id/product_uuid",
			"cat /sys/class/dmi/id/board_serial",
		}

		for _, cmd := range cmds {
			out, err := exec.Command("sh", "-c", cmd).Output()
			if err == nil {
				value := strings.TrimSpace(string(out))
				if value != "" {
					identifiers = append(identifiers, value)
				}
			}
		}
	}

	if len(identifiers) == 0 {
		return 0, fmt.Errorf("no hardware identifiers found")
	}

	// Combine all identifiers and create a hash
	combined := strings.Join(identifiers, "|")
	hash := sha256.Sum256([]byte(combined))
	// Return first 64 bits as an uint64 ==> this is the pseudo-unique identifier of the system
	return uint64(hash[7])<<56 + uint64(hash[6])<<48 + uint64(hash[5])<<40 + uint64(hash[4])<<32 + uint64(hash[3])<<24 + uint64(hash[2])<<16 + uint64(hash[1])<<8 + uint64(hash[0]), nil
}

// LoadConfig reads a JSON configuration file into the provided struct, applies
// default values from struct tags, synchronizes an optional `Version` field,
// and manages password encryption/decryption.
//
// Behavior:
//   - If the file does not exist, an empty configuration is assumed.
//   - Fields named `<Name>Password` and `<Name>SecurePassword` are treated as a
//     pair. If the plaintext password differs from the recognized marker,
//     it will be encrypted into `<Name>SecurePassword` and the plaintext field
//     will be replaced by the marker string.
//   - When `cleanConfig` is true, the function writes back a config file where
//     passwords are present as plaintext (use with care), primarily for
//     migration or inspection purposes.
//   - On successful completion and when `cleanConfig` is false, passwords are
//     decrypted in memory so callers can use the plaintext values directly.
//
// The optional `getHardwareID_func` allows overriding the hardware-ID based key
// derivation used for encryption, which is primarily intended for testing.
func LoadConfig(config interface{}, version int, path string, cleanConfig bool, getHardwareID_func ...func() (uint64, error)) error {

	var file []byte

	if len(getHardwareID_func) > 0 {
		config_init(getHardwareID_func[0])
	} else {
		config_init(secure_config_getHardwareID)
	}

	_, err := os.Stat(path)
	if !os.IsNotExist(err) {
		file, err = os.ReadFile(path)
		if err != nil {
			return fmt.Errorf(t("config.read_failed"), err)
		}
	} else {
		file = []byte("{}")
	}

	// Analyze config type
	configValue := reflect.ValueOf(config)
	if configValue.Kind() == reflect.Ptr {
		configValue = configValue.Elem()
	} else {
		return fmt.Errorf("%s", t("config.config_no_struct"))
	}
	if configValue.Kind() != reflect.Struct {
		return fmt.Errorf("%s", t("config.config_no_struct"))
	}

	if err := updateDefaultValues(configValue); err != nil {
		return fmt.Errorf(t("config.failed_defaulting"), err)
	}

	if err := json.Unmarshal(file, config); err != nil {
		return fmt.Errorf(t("config.failed_parsing"), err)
	}
	changed := false
	if err := updateVersionAndPasswords(configValue, version, &changed); err != nil {
		return fmt.Errorf(t("config.failed_checking"), err)
	}
	if cleanConfig {
		/* Decrypt passwords before writing */
		if err := decodePasswords(configValue); err != nil {
			return fmt.Errorf(t("config.failed_decode_pw"), err)
		}
		changed = true
	}
	if changed {
		configJSON, err := json.MarshalIndent(config, "", "\t")
		if err != nil {
			return fmt.Errorf(t("config.failed_build_json"), err)
		}
		if err := os.WriteFile(path, configJSON, 0644); err != nil {
			return fmt.Errorf(t("config.failed_writing"), path, err)
		}
	}
	if !cleanConfig {
		/* Decrypt passwords after writing */
		if err := decodePasswords(configValue); err != nil {
			return fmt.Errorf(t("config.failed_decode_pw"), err)
		}
	}
	return nil
}

/*
 * Password key initialization
 *
 * This involves initializing from the computer's hardware properties.
 * This makes the file unusable on another computer - this is an
 * additional security feature.
 *
 * For transferring files of the first version of this application, an old,
 * insecure key generation procedure can also be used.
 */
func config_init(getHardwareID_func func() (uint64, error)) {
	if !initialized {
		// Generate encryption key based on Hardware IS
		hardwareID, err := getHardwareID_func()
		if err != nil {
			log.Fatalf("%s", t("config.hardware_id_failed"))
		}
		randGenSeeded := mathRand.NewSource(int64(hardwareID))
		encryptionKey = make([]byte, 32)
		for i := range encryptionKey {
			encryptionKey[i] = byte(randGenSeeded.Int63() >> 16 & 0xff)
		}
		curr_lang := getCurrentLanguage()
		setLanguage("de")
		PASSWORD_IS_SECURE_de = t("config.password_message")
		setLanguage("en")
		PASSWORD_IS_SECURE_en = t("config.password_message")
		setLanguage(curr_lang)
		PASSWORD_IS_SECURE = t("config.password_message")
	}
	initialized = true
}

/*
 * Go through the structure and set the default values present
 * in the annotations
 */
func updateDefaultValues(v reflect.Value) error {
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}
	if v.Kind() != reflect.Struct {
		return nil
	}
	type_info := v.Type()
	// Iterate through all fields
	for i := 0; i < type_info.NumField(); i++ {
		field := type_info.Field(i)
		fieldValue := v.Field(i)
		if field.Type.Kind() == reflect.Struct {
			if err := updateDefaultValues(fieldValue); err != nil {
				return fmt.Errorf(t("config.default_error"), err)
			}
		} else if field.Type.Kind() == reflect.Slice {
			for i := 0; i < fieldValue.Len(); i++ {
				if fieldValue.Index(i).Kind() == reflect.Struct {
					if err := updateDefaultValues(fieldValue.Index(i)); err != nil {
						return err
					}
				}
			}
		} else {
			defaultValue, found := field.Tag.Lookup("default")
			if found {
				switch fieldValue.Kind() {
				case reflect.String:
					fieldValue.SetString(defaultValue)
				case reflect.Int, reflect.Int64:
					value, err := strconv.Atoi(defaultValue)
					if err != nil {
						return fmt.Errorf(t("config.default_error"), err)
					}
					fieldValue.SetInt(int64(value))
				case reflect.Bool:
					boolValue, err := strconv.ParseBool(defaultValue)
					if err != nil {
						return fmt.Errorf(t("config.default_error"), err)
					}
					fieldValue.SetBool(boolValue)
				default:
					return fmt.Errorf(t("config.default_unsupported"), fieldValue.Kind())
				}
			}
		}
	}
	return nil
}

/*
 * Check new content and update encrypted passwords and version as needed
 * If changes are made, the modified file will be written back at the end
 */
func updateVersionAndPasswords(v reflect.Value, version int, changed *bool) error {
	if v.Kind() == reflect.Ptr {
		//fmt.Printf("Pointer\n")
		v = v.Elem()
	}
	if v.Kind() != reflect.Struct {
		return nil
	}
	t := v.Type()
	// Iterate through all fields
	for i := 0; i < t.NumField(); i++ {

		field := t.Field(i)
		fieldValue := v.Field(i)

		// Process nested structures recursively
		if field.Type.Kind() == reflect.Struct {
			if err := updateVersionAndPasswords(fieldValue, version, changed); err != nil {
				return err
			}
		} else if field.Type.Kind() == reflect.Slice {
			//fmt.Printf("Slice[0..%d]\n", fieldValue.Len()-1)
			for i := 0; i < fieldValue.Len(); i++ {
				//fmt.Printf("Slice-Element %d:\n", i)
				if fieldValue.Index(i).Kind() == reflect.Struct {
					if err := updateVersionAndPasswords(fieldValue.Index(i), version, changed); err != nil {
						return err
					}
				} else {
					//fmt.Printf(" is '%v' (%s)\n", fieldValue.Index(i), fieldValue.Index(i).Kind().String())
				}
			}
		} else {
			// Version check
			if field.Name == "Version" {
				if fieldValue.Int() != int64(version) {
					fieldValue.SetInt(int64(version))
					*changed = true
				}
			}
			// Password handling
			if strings.HasSuffix(field.Name, "SecurePassword") {
				pw_prefix := strings.TrimSuffix(field.Name, "SecurePassword")
				for j := 0; j < t.NumField(); j++ {
					if t.Field(j).Name == pw_prefix+"Password" {
						field2Value := v.Field(j)
						if field2Value.String() != PASSWORD_IS_SECURE_de && field2Value.String() != PASSWORD_IS_SECURE_en {
							// New password found in plain text
							// New Secure_Password is calculated
							password := encrypt(field2Value.String())
							fieldValue.SetString(password)
							field2Value.SetString(PASSWORD_IS_SECURE)
							//fmt.Printf(" new value %s\n", password)
							*changed = true
						}
						break
					}
				}
			}
		}
	}
	return nil
}

/*
 * Decrypt the encrypted passwords so that the encryption is transparent in the main program.
 */
func decodePasswords(v reflect.Value) error {
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}
	if v.Kind() != reflect.Struct {
		return nil
	}
	type_info := v.Type()
	// Iterate through all fields
	for i := 0; i < type_info.NumField(); i++ {
		field := type_info.Field(i)
		fieldValue := v.Field(i)

		// Process recursively nested structures
		if field.Type.Kind() == reflect.Struct {
			if err := decodePasswords(fieldValue); err != nil {
				return err
			}
		} else if field.Type.Kind() == reflect.Slice {
			for i := 0; i < fieldValue.Len(); i++ {
				if fieldValue.Index(i).Kind() == reflect.Struct {
					if err := decodePasswords(fieldValue.Index(i)); err != nil {
						return err
					}
				}
			}
		} else {
			// Password processing
			if strings.HasSuffix(field.Name, "SecurePassword") {
				pw_prefix := strings.TrimSuffix(field.Name, "SecurePassword")
				for j := 0; j < type_info.NumField(); j++ {
					if type_info.Field(j).Name == pw_prefix+"Password" {
						field2Value := v.Field(j)
						password, err := decrypt(fieldValue.String())
						if err != nil {
							return fmt.Errorf(t("config.decrypt_failed", pw_prefix), err)
						}
						field2Value.SetString(password)
						break
					}
				}
			}
		}
	}
	return nil
}

func encrypt(text string) string {
	block, _ := aes.NewCipher(encryptionKey)
	gcm, _ := cipher.NewGCM(block)
	nonce := make([]byte, gcm.NonceSize())
	io.ReadFull(rand.Reader, nonce)
	ciphertext := gcm.Seal(nonce, nonce, []byte(text), nil)
	return base64.StdEncoding.EncodeToString(ciphertext)
}

func decrypt(text string) (string, error) {
	block, _ := aes.NewCipher(encryptionKey)
	gcm, _ := cipher.NewGCM(block)
	data, _ := base64.StdEncoding.DecodeString(text)
	nonceSize := gcm.NonceSize()
	nonce, ciphertext := data[:nonceSize], data[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	return string(plaintext), err
}

//ChangeLog:
// 24.11.25	1.2.0	Included reference to php variant
