package sconfig

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"
)

// Known-Answer-Tests: these values were captured with the code of version
// 2.0.8.62 (before the v2 rework) and must never change, otherwise existing
// encrypted config files become unreadable.
const (
	katV1Identifiers0 = "SERIAL123"
	katV1Identifiers1 = "00:11:22:33:44:55"
	katV1HardwareID   = uint64(11783701102412653513)
	katV1Key          = "74f9a997f4d22e17fc417dcb5ddc4deb1641ea3492c30c3f5c873925740fc55a"
	katV1Key12345     = "691ae7604c2f2f1df37605f03fc6dee5c3e42044dee335643283a4c5db9c7a36"
	// katV1Cipher12345 is "legacy-secret" encrypted by version 2.0.8.62 with
	// the hardware-ID override 12345.
	katV1Cipher12345 = "6kebxphQI861K+aj3bgEJFJw6HNHmsmwj2J/DLDbZVYnWUPmYjzK048="

	// Shared v2 vectors (identical in php/tests/EnvLoaderTest.php).
	katV2Key         = "e28160dbd3a0ec53399e2739e768f142d998295e82e71d4ae8b6127d8c1e651c"
	katV2KeyOverride = "81148b67e0975a57c71c270d555f4aa1d60e22a572b984e119f7eefa81e6000f"
)

func override12345() (uint64, error) { return 12345, nil }

func TestKAT_LegacyHardwareIDAndKey(ts *testing.T) {
	id, combined := hardwareIDFromIdentifiersV1([]string{katV1Identifiers0, katV1Identifiers1})
	if combined != katV1Identifiers1+"|"+katV1Identifiers0 {
		ts.Errorf("identifiers must be sorted before joining, got %q", combined)
	}
	if id != katV1HardwareID {
		ts.Fatalf("legacy hardware ID changed: got %d, want %d", id, katV1HardwareID)
	}
	if got := hex.EncodeToString(deriveKeyV1(id)); got != katV1Key {
		ts.Errorf("legacy key changed: got %s, want %s", got, katV1Key)
	}
	if got := hex.EncodeToString(deriveKeyV1(12345)); got != katV1Key12345 {
		ts.Errorf("legacy key(12345) changed: got %s, want %s", got, katV1Key12345)
	}
}

func TestKAT_V2Key(ts *testing.T) {
	key, err := deriveKeyV2(map[string]string{
		sourceSMBIOSUUID:  "4c4c4544-0042-3510-8051-b2c04f4a3732",
		sourceMachineGUID: "1b4e28ba-2fa1-11d2-883f-0016d3cca427",
	})
	if err != nil {
		ts.Fatal(err)
	}
	if got := hex.EncodeToString(key); got != katV2Key {
		ts.Errorf("v2 key changed: got %s, want %s", got, katV2Key)
	}
	key, _ = deriveKeyV2(map[string]string{sourceOverride: "12345"})
	if got := hex.EncodeToString(key); got != katV2KeyOverride {
		ts.Errorf("v2 override key changed: got %s, want %s", got, katV2KeyOverride)
	}
	if _, err := deriveKeyV2(nil); !errors.Is(err, errNoMachineSource) {
		ts.Errorf("expected errNoMachineSource, got %v", err)
	}
}

// TestLegacyValueIsMigrated loads a config written by version 2.0.8.62 and
// checks that it decrypts and is rewritten in the v2 format.
func TestLegacyValueIsMigrated(ts *testing.T) {
	ResetForTest()
	defer ResetForTest()
	dir := testExeRoot(ts)
	path := filepath.Join(dir, "legacy.json")
	content := `{"version":1,"database_password":"Enter new password here","database_secure_password":"` + katV1Cipher12345 + `"}`
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		ts.Fatal(err)
	}
	cfg := &TestConfig{}
	if err := LoadConfig(cfg, 1, path, false, false, override12345); err != nil {
		ts.Fatalf("legacy config must load: %v", err)
	}
	if cfg.DatabasePassword != "legacy-secret" {
		ts.Fatalf("expected legacy-secret, got %q", cfg.DatabasePassword)
	}
	raw, _ := os.ReadFile(path)
	var onDisk TestConfig
	if err := json.Unmarshal(raw, &onDisk); err != nil {
		ts.Fatal(err)
	}
	if !strings.HasPrefix(onDisk.DatabaseSecurePassword, "v2:override:") {
		ts.Fatalf("expected migrated v2 value, got %q", onDisk.DatabaseSecurePassword)
	}
	cfg2 := &TestConfig{}
	if err := LoadConfig(cfg2, 1, path, false, false); err != nil {
		ts.Fatalf("migrated config must load: %v", err)
	}
	if cfg2.DatabasePassword != "legacy-secret" {
		ts.Errorf("expected legacy-secret after migration, got %q", cfg2.DatabasePassword)
	}
}

func TestV2SourceMissingIsReported(ts *testing.T) {
	ResetForTest()
	defer ResetForTest()
	dir := testExeRoot(ts)
	path := filepath.Join(dir, "missing.json")
	content := `{"version":1,"database_password":"Enter new password here","database_secure_password":"v2:machineid:6kebxphQI861K+aj3bgEJFJw6HNHmsmwj2J/DLDbZVYnWUPmYjzK048="}`
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		ts.Fatal(err)
	}
	err := LoadConfig(&TestConfig{}, 1, path, false, false, override12345)
	if err == nil || !strings.Contains(err.Error(), "machineid") {
		ts.Fatalf("expected an error naming the missing source, got %v", err)
	}
}

func TestParseV2ValueRejectsMalformed(ts *testing.T) {
	for _, v := range []string{"v2:", "v2::abc", "v2:Upper:abc", "v2:a b:abc", "v2:abc",
		"v2:smbiosuuid,machineguid:abc", "v2:a,a:abc", "v2:a,b,c,d,e,f,g,h,i:abc"} {
		if _, _, err := parseV2Value(v); !errors.Is(err, errInvalidCiphertext) {
			ts.Errorf("%q: expected errInvalidCiphertext, got %v", v, err)
		}
	}
	names, b64, err := parseV2Value("v2:machineguid,smbiosuuid:QUJD")
	if err != nil || len(names) != 2 || b64 != "QUJD" {
		ts.Errorf("unexpected parse result %v %q %v", names, b64, err)
	}
}

func TestSourceNormalization(ts *testing.T) {
	cases := map[string]string{
		" {1B4E28BA-2FA1-11D2-883F-0016D3CCA427}\r\n": "1b4e28ba-2fa1-11d2-883f-0016d3cca427",
		"00000000-0000-0000-0000-000000000000":        "",
		"03000200-0400-0500-0006-000700080009":        "",
		"not-a-uuid":                                  "",
	}
	for in, want := range cases {
		if got := normalizeUUID(in); got != want {
			ts.Errorf("normalizeUUID(%q) = %q, want %q", in, got, want)
		}
	}
	if got := normalizeMachineID("0123456789ABCDEF0123456789abcdef\n"); got != "0123456789abcdef0123456789abcdef" {
		ts.Errorf("normalizeMachineID: got %q", got)
	}
	if normalizeMachineID("00000000000000000000000000000000") != "" {
		ts.Error("all-zero machine-id must be rejected")
	}
	reg := "\r\nHKEY_LOCAL_MACHINE\\SOFTWARE\\Microsoft\\Cryptography\r\n    MachineGuid    REG_SZ    1b4e28ba-2fa1-11d2-883f-0016d3cca427\r\n\r\n"
	if got := parseRegQueryValue(reg, "MachineGuid"); got != "1b4e28ba-2fa1-11d2-883f-0016d3cca427" {
		ts.Errorf("parseRegQueryValue: got %q", got)
	}
	ioreg := `  | |   "IOPlatformUUID" = "1B4E28BA-2FA1-11D2-883F-0016D3CCA427"`
	if got := parseIoregPlatformUUID(ioreg); got != "1B4E28BA-2FA1-11D2-883F-0016D3CCA427" {
		ts.Errorf("parseIoregPlatformUUID: got %q", got)
	}
}

func TestCollectMachineSourcesV2(ts *testing.T) {
	sources, err := collectMachineSourcesV2()
	if err != nil {
		ts.Fatalf("collecting v2 sources failed: %v", err)
	}
	for name, value := range sources {
		if !sourceNamePattern.MatchString(name) || value == "" {
			ts.Errorf("invalid source %q=%q", name, value)
		}
	}
	ts.Logf("v2 sources on this system: %v", sortedSourceNames(sources))
}

type pointerSub struct {
	DBPassword       string `json:"db_password"`
	DBSecurePassword string `json:"db_secure_password"`
	Port             int    `json:"port" default:"22"`
}

type pointerConfig struct {
	Version int                    `json:"version"`
	Ptr     *pointerSub            `json:"ptr"`
	List    []*pointerSub          `json:"list"`
	Arr     [1]pointerSub          `json:"arr"`
	Map     map[string]pointerSub  `json:"map"`
	PtrMap  map[string]*pointerSub `json:"ptr_map"`
	Items   []pointerSub           `json:"items"`
}

// TestPasswordsInPointersMapsArrays checks CR-5 (no plaintext left in file).
func TestPasswordsInPointersMapsArrays(ts *testing.T) {
	ResetForTest()
	defer ResetForTest()
	dir := testExeRoot(ts)
	path := filepath.Join(dir, "pointers.json")
	cfg := &pointerConfig{
		Ptr:    &pointerSub{DBPassword: "p-ptr"},
		List:   []*pointerSub{{DBPassword: "p-list"}},
		Arr:    [1]pointerSub{{DBPassword: "p-arr"}},
		Map:    map[string]pointerSub{"a": {DBPassword: "p-map"}},
		PtrMap: map[string]*pointerSub{"b": {DBPassword: "p-ptrmap"}},
	}
	if err := LoadConfig(cfg, 1, path, false, false, override12345); err != nil {
		ts.Fatal(err)
	}
	raw, _ := os.ReadFile(path)
	for _, secret := range []string{"p-ptr", "p-list", "p-arr", "p-map", "p-ptrmap"} {
		if strings.Contains(string(raw), `"`+secret+`"`) {
			ts.Errorf("plaintext %q left in file", secret)
		}
	}
	if cfg.Ptr.DBPassword != "p-ptr" || cfg.List[0].DBPassword != "p-list" || cfg.Arr[0].DBPassword != "p-arr" ||
		cfg.Map["a"].DBPassword != "p-map" || cfg.PtrMap["b"].DBPassword != "p-ptrmap" {
		ts.Errorf("passwords not decrypted in memory: %+v", cfg)
	}
	cfg2 := &pointerConfig{}
	if err := LoadConfig(cfg2, 1, path, false, false); err != nil {
		ts.Fatal(err)
	}
	if cfg2.Ptr.DBPassword != "p-ptr" || cfg2.Map["a"].DBPassword != "p-map" {
		ts.Errorf("reload did not decrypt: %+v", cfg2)
	}
}

// TestDefaultsForDecodedElements checks CR-18.
func TestDefaultsForDecodedElements(ts *testing.T) {
	ResetForTest()
	defer ResetForTest()
	dir := testExeRoot(ts)
	path := filepath.Join(dir, "defaults.json")
	content := `{"items":[{"port":2200},{}],"ptr":{},"map":{"x":{}},"list":[{}]}`
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		ts.Fatal(err)
	}
	cfg := &pointerConfig{}
	if err := LoadConfig(cfg, 1, path, false, false, override12345); err != nil {
		ts.Fatal(err)
	}
	if len(cfg.Items) != 2 || cfg.Items[0].Port != 2200 || cfg.Items[1].Port != 22 {
		ts.Errorf("slice element defaults wrong: %+v", cfg.Items)
	}
	if cfg.Ptr == nil || cfg.Ptr.Port != 22 {
		ts.Errorf("pointer default wrong: %+v", cfg.Ptr)
	}
	if cfg.Map["x"].Port != 22 {
		ts.Errorf("map element default wrong: %+v", cfg.Map)
	}
	if len(cfg.List) != 1 || cfg.List[0].Port != 22 {
		ts.Errorf("pointer slice element default wrong: %+v", cfg.List)
	}
}

func TestDefaultTypes(ts *testing.T) {
	type typed struct {
		D   time.Duration `default:"1m30s"`
		U   uint16        `default:"8080"`
		F   float64       `default:"0.5"`
		I8  int8          `default:"-5"`
		Bad []string      `default:"x"`
	}
	v := &typed{}
	err := applyDefaults(reflect.ValueOf(v))
	if err == nil {
		ts.Error("expected error for unsupported default on slice")
	}
	if v.D != 90*time.Second || v.U != 8080 || v.F != 0.5 || v.I8 != -5 {
		ts.Errorf("typed defaults wrong: %+v", v)
	}
	type overflow struct {
		I8 int8 `default:"300"`
	}
	if err := applyDefaults(reflect.ValueOf(&overflow{})); err == nil {
		ts.Error("expected overflow error for int8 default 300")
	}
}

// TestNestedVersionIsKept checks CR-10.
func TestNestedVersionIsKept(ts *testing.T) {
	ResetForTest()
	defer ResetForTest()
	type plugin struct {
		Version string `json:"version"`
	}
	type cfgT struct {
		Version int      `json:"version"`
		Plugins []plugin `json:"plugins"`
		Sub     struct {
			Version int `json:"version"`
		} `json:"sub"`
	}
	dir := testExeRoot(ts)
	path := filepath.Join(dir, "nested_version.json")
	if err := os.WriteFile(path, []byte(`{"version":1,"plugins":[{"version":"1.2"}],"sub":{"version":7}}`), 0600); err != nil {
		ts.Fatal(err)
	}
	cfg := &cfgT{}
	if err := LoadConfig(cfg, 3, path, false, false, override12345); err != nil {
		ts.Fatal(err)
	}
	if cfg.Version != 3 || cfg.Sub.Version != 7 || cfg.Plugins[0].Version != "1.2" {
		ts.Errorf("only the top-level Version may change: %+v", cfg)
	}
}

func TestLoadConfig_NoHardwareIDIsErrorNotExit(ts *testing.T) {
	ResetForTest()
	defer ResetForTest()
	dir := testExeRoot(ts)
	failing := func() (uint64, error) { return 0, errors.New("boom") }
	// Without passwords no key is needed at all.
	if err := LoadConfig(&TestConfig{}, 1, filepath.Join(dir, "a.json"), false, false, failing); err != nil {
		ts.Fatalf("config without passwords must load: %v", err)
	}
	err := LoadConfig(&TestConfig{DatabasePassword: "x"}, 1, filepath.Join(dir, "b.json"), false, false, failing)
	if err == nil || !strings.Contains(err.Error(), "boom") {
		ts.Fatalf("expected error with cause, got %v", err)
	}
}

func TestNewFilesArePrivateAndWritesAtomic(ts *testing.T) {
	ResetForTest()
	defer ResetForTest()
	dir := testExeRoot(ts)
	path := filepath.Join(dir, "private.json")
	if err := LoadConfig(&TestConfig{DatabasePassword: "x"}, 1, path, true, false, override12345); err != nil {
		ts.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		ts.Fatal(err)
	}
	if runtime.GOOS != "windows" && info.Mode().Perm() != 0600 {
		ts.Errorf("new config file must be 0600, got %o", info.Mode().Perm())
	}
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if strings.Contains(e.Name(), ".tmp-") {
			ts.Errorf("temporary file left behind: %s", e.Name())
		}
	}
}

func TestPathCheckResolvesSymlinks(ts *testing.T) {
	ResetForTest()
	defer ResetForTest()
	root := testExeRoot(ts)
	outside := ts.TempDir()
	link := filepath.Join(root, "link")
	if err := os.Symlink(outside, link); err != nil {
		if runtime.GOOS != "windows" {
			ts.Skipf("cannot create symlink: %v", err)
		}
		// Junctions need no special privilege on Windows.
		if out, jerr := exec.Command("cmd", "/C", "mklink", "/J", link, outside).CombinedOutput(); jerr != nil {
			ts.Skipf("cannot create symlink (%v) or junction (%v: %s)", err, jerr, out)
		}
	}
	// The CWD must not be an allowed base for this check.
	oldWd, _ := os.Getwd()
	if err := os.Chdir(root); err != nil {
		ts.Fatal(err)
	}
	defer func() { _ = os.Chdir(oldWd) }()
	if _, err := resolveConfigPath(filepath.Join(link, "config.json")); err == nil {
		ts.Error("a symlink inside the base pointing outside must be rejected")
	}
}

func TestSetLocalesDir(ts *testing.T) {
	dir := ts.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "en.json"), []byte(`{"items":[1,2]}`), 0600); err != nil {
		ts.Fatal(err)
	}
	if err := SetLocalesDir(dir); err == nil {
		ts.Error("invalid locale file must return an error, not panic")
	}
	good := ts.TempDir()
	if err := os.WriteFile(filepath.Join(good, "en.json"), []byte(`{"config.password_message":"changed","test.extra":"extra text"}`), 0600); err != nil {
		ts.Fatal(err)
	}
	if err := SetLocalesDir(good); err != nil {
		ts.Fatalf("valid locale dir: %v", err)
	}
	if markerText("en") != "Enter new password here" {
		ts.Errorf("external locales must not change the marker, got %q", markerText("en"))
	}
	old := getCurrentLanguage()
	setLanguage("en")
	defer setLanguage(old)
	if got := t("test.extra"); got != "extra text" {
		ts.Errorf("external translation not used: %q", got)
	}
}

func TestDetectLanguagePrecedence(ts *testing.T) {
	ts.Setenv("LANG", "en_US.UTF-8")
	ts.Setenv("LC_MESSAGES", "")
	ts.Setenv("LC_ALL", "de_DE.UTF-8")
	if got := detectLanguage(); got != "de" {
		ts.Errorf("LC_ALL must win over LANG, got %s", got)
	}
}

func TestRunCommandTimeout(ts *testing.T) {
	name, args := "sleep", []string{"5"}
	if runtime.GOOS == "windows" {
		name, args = "ping", []string{"-n", "6", "127.0.0.1"}
	}
	_, err := runCommandTimeout(200*time.Millisecond, name, args...)
	if !errors.Is(err, errCommandTimeout) {
		ts.Fatalf("expected errCommandTimeout, got %v", err)
	}
}

func TestParallelLoadConfig(ts *testing.T) {
	ResetForTest()
	defer ResetForTest()
	dir := testExeRoot(ts)
	var wg sync.WaitGroup
	errs := make(chan error, 8)
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			cfg := &TestConfig{DatabasePassword: "parallel"}
			path := filepath.Join(dir, "parallel_"+string(rune('a'+i))+".json")
			if err := LoadConfig(cfg, 1, path, false, false, override12345); err != nil {
				errs <- err
				return
			}
			if cfg.DatabasePassword != "parallel" {
				errs <- errors.New("wrong password " + cfg.DatabasePassword)
			}
		}(i)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		ts.Error(err)
	}
}

func TestPHPLocalesInSync(ts *testing.T) {
	for _, name := range []string{"en.json", "de.json"} {
		want, err := os.ReadFile(filepath.Join("locales", name))
		if err != nil {
			ts.Fatal(err)
		}
		got, err := os.ReadFile(filepath.Join("php", "locales", name))
		if err != nil {
			ts.Fatalf("php/locales/%s missing: %v", name, err)
		}
		if string(got) != string(want) {
			ts.Errorf("php/locales/%s differs from locales/%s (copy it)", name, name)
		}
	}
}

// TestDecryptsPHPValue checks that a v2 value written by the PHP
// implementation (override 12345) decrypts in Go (CR-16).
func TestDecryptsPHPValue(ts *testing.T) {
	ResetForTest()
	defer ResetForTest()
	config_init(override12345, false)
	const phpValue = "v2:override:zStszyITV61ycwBY/qA9gUqe9sUB/bcEbTfcWrPubtFyDSkQN+aMxnk="
	plain, err := decrypt(phpValue, "shared")
	if err != nil || plain != "shared-secret" {
		ts.Fatalf("PHP value must decrypt to shared-secret, got %q, %v", plain, err)
	}
}

// TestFieldBinding checks SA-10: a value cannot be moved to another field.
func TestFieldBinding(ts *testing.T) {
	ResetForTest()
	defer ResetForTest()
	config_init(override12345, false)
	value, err := encrypt("bound", "Database")
	if err != nil {
		ts.Fatal(err)
	}
	if plain, err := decrypt(value, "DATABASE"); err != nil || plain != "bound" {
		ts.Fatalf("Go and PHP field names must give the same AAD: %q %v", plain, err)
	}
	if _, err := decrypt(value, "API"); err == nil {
		ts.Fatal("a value moved to another field must not decrypt")
	}
	if string(fieldAAD("DB_Main")) != "sconfig:dbmain" {
		ts.Errorf("unexpected AAD %q", fieldAAD("DB_Main"))
	}
}

func TestKeyFileAndUpgrade(ts *testing.T) {
	ResetForTest()
	dir := testExeRoot(ts)
	defer func() {
		_ = SetKeyFile("")
		ResetForTest()
	}()
	path := filepath.Join(dir, "keyfile.json")
	cfg := &TestConfig{DatabasePassword: "with-keyfile"}
	if err := LoadConfig(cfg, 1, path, false, false, override12345); err != nil {
		ts.Fatal(err)
	}
	keyPath := filepath.Join(dir, "sconfig.key")
	if err := SetKeyFile(keyPath); err != nil {
		ts.Fatalf("SetKeyFile: %v", err)
	}
	data, err := os.ReadFile(keyPath)
	if err != nil || len(data) != keyFileNewSize {
		ts.Fatalf("key file not created correctly: %d bytes, %v", len(data), err)
	}
	if runtime.GOOS != "windows" {
		if info, _ := os.Stat(keyPath); info.Mode().Perm() != 0600 {
			ts.Errorf("key file must be 0600, got %o", info.Mode().Perm())
		}
	}
	// The existing value (override only) is upgraded to override+keyfile.
	cfg2 := &TestConfig{}
	if err := LoadConfig(cfg2, 1, path, false, false); err != nil {
		ts.Fatal(err)
	}
	raw, _ := os.ReadFile(path)
	if !strings.Contains(string(raw), "v2:keyfile,override:") {
		ts.Fatalf("value not upgraded to the key file: %s", raw)
	}
	// Without the key file the value cannot be decrypted any more.
	if err := SetKeyFile(""); err != nil {
		ts.Fatal(err)
	}
	if err := LoadConfig(&TestConfig{}, 1, path, false, false); err == nil || !strings.Contains(err.Error(), "keyfile") {
		ts.Fatalf("expected missing keyfile source, got %v", err)
	}
	// A too small key file is rejected.
	small := filepath.Join(dir, "small.key")
	_ = os.WriteFile(small, []byte("short"), 0600)
	if err := SetKeyFile(small); err == nil {
		ts.Error("too small key file must be rejected")
	}
}

func TestRejectLegacyAndWarnings(ts *testing.T) {
	ResetForTest()
	var warnings []string
	SetWarningHandler(func(m string) { warnings = append(warnings, m) })
	defer func() {
		SetWarningHandler(nil)
		SetRejectLegacy(false)
		ResetForTest()
	}()
	dir := testExeRoot(ts)
	path := filepath.Join(dir, "legacy_strict.json")
	content := `{"version":1,"database_password":"Enter new password here","database_secure_password":"` + katV1Cipher12345 + `"}`
	_ = os.WriteFile(path, []byte(content), 0600)
	SetRejectLegacy(true)
	if err := LoadConfig(&TestConfig{}, 1, path, false, false, override12345); err == nil {
		ts.Fatal("strict mode must refuse legacy values")
	}
	SetRejectLegacy(false)
	ResetForTest()
	if err := LoadConfig(&TestConfig{}, 1, path, false, false, override12345); err != nil {
		ts.Fatal(err)
	}
	if len(warnings) != 1 || !strings.Contains(warnings[0], "1") {
		ts.Errorf("expected one migration warning, got %q", warnings)
	}
}

func TestFileTooLarge(ts *testing.T) {
	dir := ts.TempDir()
	path := filepath.Join(dir, "big.json")
	_ = os.WriteFile(path, make([]byte, 101), 0600)
	if _, err := readFileLimited(path, 100); err == nil {
		ts.Error("files above the limit must be rejected")
	}
	if data, err := readFileLimited(path, 101); err != nil || len(data) != 101 {
		ts.Errorf("file at the limit must be read: %d %v", len(data), err)
	}
}

func TestAtomicWriteKeepsSymlinkAndWarns(ts *testing.T) {
	if runtime.GOOS == "windows" {
		ts.Skip("symlink and Unix permission semantics")
	}
	ResetForTest()
	var warnings []string
	SetWarningHandler(func(m string) { warnings = append(warnings, m) })
	defer func() { SetWarningHandler(nil); ResetForTest() }()
	dir := testExeRoot(ts)
	real := filepath.Join(dir, "real.json")
	_ = os.WriteFile(real, []byte(`{}`), 0644)
	link := filepath.Join(dir, "link.json")
	if err := os.Symlink(real, link); err != nil {
		ts.Skip(err)
	}
	if err := LoadConfig(&TestConfig{DatabasePassword: "x"}, 1, link, false, false, override12345); err != nil {
		ts.Fatal(err)
	}
	if info, err := os.Lstat(link); err != nil || info.Mode()&os.ModeSymlink == 0 {
		ts.Error("symlink must be kept")
	}
	if len(warnings) == 0 {
		ts.Error("expected a warning for a world-readable config file")
	}
}

func TestNewFileReadableOnWindows(ts *testing.T) {
	ResetForTest()
	defer ResetForTest()
	dir := testExeRoot(ts)
	path := filepath.Join(dir, "acl.json")
	if err := LoadConfig(&TestConfig{DatabasePassword: "x"}, 1, path, false, false, override12345); err != nil {
		ts.Fatal(err)
	}
	if err := LoadConfig(&TestConfig{}, 1, path, false, false); err != nil {
		ts.Fatalf("restricted file must stay readable and writable for the owner: %v", err)
	}
	if err := UpdateConfig(&TestConfig{DatabasePassword: "y"}, path); err != nil {
		ts.Fatalf("replacing an existing file failed: %v", err)
	}
}
