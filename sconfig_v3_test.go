package sconfig

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Shared v3 vectors (identical in php/tests/EnvLoaderV3Test.php). They must
// never change, otherwise existing v3 values become unreadable.
const (
	katV3Key         = "242105607bde99e45b89b953c3306952ad59ceba37219e87a44454bfacc97673"
	katV3KeyOverride = "4b4610f288c50c5ff7ad1c006b276ecab575313dc7577d32d4f87fff1bcebf89"
	// katV3Value is "shared-secret" for field "shared" with override 12345.
	katV3Value = "v3:override:Fw37IF2vJefENHXxr+lHOvMi9ORfmR0Ju8QpGM4D0xuFEzVkZghTGQw="
	// katV2ValueDatabase is "v2-secret" for field "Database" (v2, override
	// 12345) as written by version 3.0.0.
	katV2ValueDatabase = "v2:override:cYuA/Ekli4qbh33oldf91p83HqsNJTolW+rBI2FT2b1JsCfoyw=="
)

// katV3Sources are the sources of katV3Key.
var katV3Sources = map[string]string{
	sourceFSID:        "68ed38fecd67cf6",
	sourceHostname:    "build-01.example.com",
	sourceMachineGUID: "1b4e28ba-2fa1-11d2-883f-0016d3cca427",
	sourceMachType:    "amd64",
	sourceSMBIOSUUID:  "4c4c4544-0042-3510-8051-b2c04f4a3732",
}

func TestKAT_V3Key(ts *testing.T) {
	key, err := deriveKeyV3(katV3Sources)
	if err != nil {
		ts.Fatal(err)
	}
	if got := hex.EncodeToString(key); got != katV3Key {
		ts.Errorf("v3 key changed: got %s, want %s", got, katV3Key)
	}
	key, _ = deriveKeyV3(map[string]string{sourceOverride: "12345"})
	if got := hex.EncodeToString(key); got != katV3KeyOverride {
		ts.Errorf("v3 override key changed: got %s, want %s", got, katV3KeyOverride)
	}
	// v2 and v3 must give different keys for the same sources
	key2, _ := deriveKeyV2(map[string]string{sourceOverride: "12345"})
	if hex.EncodeToString(key2) == katV3KeyOverride {
		ts.Error("v2 and v3 keys must differ")
	}
}

func TestDecryptsV3Vector(ts *testing.T) {
	ResetForTest()
	defer ResetForTest()
	config_init(override12345, false)
	plain, err := decrypt(katV3Value, "SHARED")
	if err != nil || plain != "shared-secret" {
		ts.Fatalf("v3 vector must decrypt to shared-secret, got %q, %v", plain, err)
	}
	// written by the PHP implementation (field PHP_SHARED, override 12345)
	const phpValue = "v3:override:IUR7pS5HB9pPeEd0uek9iSUFiNlVwIAgU5ypgPsYpv57U2NoDLU="
	plain, err = decrypt(phpValue, "PhpShared")
	if err != nil || plain != "php-secret" {
		ts.Fatalf("PHP v3 value must decrypt to php-secret, got %q, %v", plain, err)
	}
}

func TestParseV3ValueRejectsMalformed(ts *testing.T) {
	for _, v := range []string{"v3:", "v3::abc", "v3:Upper:abc", "v3:machtype,hostname:abc", "v3:abc"} {
		if _, _, err := parseV3Value(v); !errors.Is(err, errInvalidCiphertext) {
			ts.Errorf("%q: expected errInvalidCiphertext, got %v", v, err)
		}
	}
	names, b64, err := parseV3Value("v3:fsid,hostname,machtype:QUJD")
	if err != nil || len(names) != 3 || b64 != "QUJD" {
		ts.Errorf("unexpected parse result %v %q %v", names, b64, err)
	}
}

// TestV2ValueIsMigratedToV3 loads a v2 value written by version 3.0.0 and
// checks the silent migration to v3.
func TestV2ValueIsMigratedToV3(ts *testing.T) {
	ResetForTest()
	var warnings []string
	SetWarningHandler(func(m string) { warnings = append(warnings, m) })
	defer func() { SetWarningHandler(nil); ResetForTest() }()
	dir := testExeRoot(ts)
	path := filepath.Join(dir, "v2.json")
	content := `{"version":1,"database_password":"Enter new password here","database_secure_password":"` + katV2ValueDatabase + `"}`
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		ts.Fatal(err)
	}
	cfg := &TestConfig{}
	if err := LoadConfig(cfg, 1, path, false, false, override12345); err != nil {
		ts.Fatalf("v2 config must load: %v", err)
	}
	if cfg.DatabasePassword != "v2-secret" {
		ts.Fatalf("expected v2-secret, got %q", cfg.DatabasePassword)
	}
	raw, _ := os.ReadFile(path)
	var onDisk TestConfig
	if err := json.Unmarshal(raw, &onDisk); err != nil {
		ts.Fatal(err)
	}
	if !strings.HasPrefix(onDisk.DatabaseSecurePassword, "v3:override:") {
		ts.Fatalf("expected migrated v3 value, got %q", onDisk.DatabaseSecurePassword)
	}
	if len(warnings) != 0 {
		ts.Errorf("v2 migration must be silent, got %q", warnings)
	}
}

func TestV3SourceNormalization(ts *testing.T) {
	hostnames := map[string]string{
		" Build-01.Example.COM\r\n": "build-01.example.com",
		"NOAH":                      "noah",
		"localhost":                 "",
		"LocalHost.LocalDomain":     "",
		"bad name":                  "",
		"müller":                    "",
		"":                          "",
		strings.Repeat("a", 256):    "",
	}
	for in, want := range hostnames {
		if got := normalizeHostname(in); got != want {
			ts.Errorf("normalizeHostname(%q) = %q, want %q", in, got, want)
		}
	}
	machTypes := map[string]string{
		"x86_64\n": "amd64", "AMD64": "amd64", "aarch64": "arm64", "ARM64": "arm64",
		"i686": "386", "x86": "386", "armv7l": "arm", "riscv64": "riscv64",
		"ppc64le": "ppc64le", "unknown": "", "bad value": "", "": "",
	}
	for in, want := range machTypes {
		if got := normalizeMachType(in); got != want {
			ts.Errorf("normalizeMachType(%q) = %q, want %q", in, got, want)
		}
	}
	fsids := map[string]string{
		"68ed38fecd67cf6\n": "68ed38fecd67cf6", "00EA5DC109": "ea5dc109",
		"0": "", "": "", "xyz": "", "12345678901234567": "",
	}
	for in, want := range fsids {
		if got := normalizeFSID(in); got != want {
			ts.Errorf("normalizeFSID(%q) = %q, want %q", in, got, want)
		}
	}
	if formatFSID(0) != "" || formatFSID(0xea5dc109) != "ea5dc109" {
		ts.Error("formatFSID")
	}
}

func TestCollectMachineSourcesV3(ts *testing.T) {
	sources, err := collectMachineSourcesV3()
	if err != nil {
		ts.Fatalf("collecting v3 sources failed: %v", err)
	}
	for name, value := range sources {
		if !sourceNamePattern.MatchString(name) || value == "" {
			ts.Errorf("invalid source %q=%q", name, value)
		}
	}
	if sources[sourceMachType] == "" {
		ts.Error("machtype must be available on every supported system")
	}
	fsid := configFileSystemID(filepath.Join(ts.TempDir(), "missing", "config.json"))
	if fsid == "" {
		ts.Error("fsid of the temp directory must be readable")
	}
	ts.Logf("v3 sources on this system: %v, fsid %s", sortedSourceNames(sources), fsid)
}

// TestV3BindsHostnameAndFileSystem uses a fixed machine-id, a replaced host
// name and file system ID and the real machine type.
func TestV3BindsHostnameAndFileSystem(ts *testing.T) {
	ResetForTest()
	origHost, origFS, origV2 := hostnameFunc, fileSystemIDFunc, machineSourcesV2Func
	// fixed strong source, so the test also runs in containers without one
	machineSourcesV2Func = func() (map[string]string, error) {
		return map[string]string{sourceMachineID: "0123456789abcdef0123456789abcdef"}, nil
	}
	host := "host-a"
	fsids := map[string]uint64{}
	hostnameFunc = func() (string, error) { return host, nil }
	fileSystemIDFunc = func(path string) (uint64, error) {
		for prefix, id := range fsids {
			if strings.HasPrefix(path, prefix) {
				return id, nil
			}
		}
		return 0xaaa, nil
	}
	defer func() {
		hostnameFunc, fileSystemIDFunc, machineSourcesV2Func = origHost, origFS, origV2
		ResetForTest()
	}()
	dir := testExeRoot(ts)
	path := filepath.Join(dir, "bound.json")
	cfg := &TestConfig{DatabasePassword: "bound-secret"}
	if err := LoadConfig(cfg, 1, path, false, false); err != nil {
		ts.Fatal(err)
	}
	raw, _ := os.ReadFile(path)
	var onDisk TestConfig
	_ = json.Unmarshal(raw, &onDisk)
	names, _, err := parseV3Value(onDisk.DatabaseSecurePassword)
	if err != nil {
		ts.Fatalf("expected a v3 value: %v (%q)", err, onDisk.DatabaseSecurePassword)
	}
	for _, want := range []string{sourceFSID, sourceHostname, sourceMachType} {
		if !strings.Contains(","+strings.Join(names, ",")+",", ","+want+",") {
			ts.Errorf("source %s missing in %v", want, names)
		}
	}

	// A backup on another file system is written with its own fsid; a clean
	// backup must still decrypt the values of the loaded file.
	backupDir := filepath.Join(dir, "otherfs")
	_ = os.Mkdir(backupDir, 0700)
	fsids[backupDir] = 0xbbb
	if err := UpdateConfig(cfg, filepath.Join(backupDir, "backup.json")); err != nil {
		ts.Fatalf("backup on another file system: %v", err)
	}
	if err := UpdateConfig(cfg, filepath.Join(backupDir, "clean.json"), true); err != nil {
		ts.Fatalf("clean backup on another file system: %v", err)
	}
	ResetForTest()
	if err := LoadConfig(&TestConfig{}, 1, filepath.Join(backupDir, "backup.json"), false, false); err != nil {
		ts.Fatalf("backup must load on its file system: %v", err)
	}

	// The file moved to another file system: key mismatch with a hint.
	ResetForTest()
	fsids[dir] = 0xccc
	err = LoadConfig(&TestConfig{}, 1, path, false, false)
	if err == nil || !strings.Contains(err.Error(), t("config.v3_key_mismatch")) {
		ts.Fatalf("expected a key mismatch after a file system change, got %v", err)
	}
	delete(fsids, dir)

	// The computer was renamed.
	ResetForTest()
	host = "host-b"
	if err := LoadConfig(&TestConfig{}, 1, path, false, false); err == nil {
		ts.Fatal("a renamed computer must not decrypt the values")
	}
	host = "host-a"
	ResetForTest()
	loaded := &TestConfig{}
	if err := LoadConfig(loaded, 1, path, false, false); err != nil || loaded.DatabasePassword != "bound-secret" {
		ts.Fatalf("original identity must decrypt again: %q %v", loaded.DatabasePassword, err)
	}
}

// withIdentityHooks replaces host name, file system ID and the v2 sources
// for one test and restores them afterwards.
func withIdentityHooks(ts *testing.T, host *string, fsid *uint64, v2 map[string]string) {
	origHost, origFS, origV2 := hostnameFunc, fileSystemIDFunc, machineSourcesV2Func
	hostnameFunc = func() (string, error) { return *host, nil }
	fileSystemIDFunc = func(string) (uint64, error) { return *fsid, nil }
	machineSourcesV2Func = func() (map[string]string, error) {
		out := map[string]string{}
		for k, v := range v2 {
			out[k] = v
		}
		return out, nil
	}
	ts.Cleanup(func() {
		hostnameFunc, fileSystemIDFunc, machineSourcesV2Func = origHost, origFS, origV2
		SetAllowWeakIdentity(false)
		SetUseHostname(true)
		SetUseFileSystemID(true)
		SetWarningHandler(nil)
		_ = SetKeyFile("")
		ResetForTest()
	})
}

// TestWeakIdentityIsRefused checks SA-17: without a strong source new values
// are refused unless a weak identity is allowed explicitly.
func TestWeakIdentityIsRefused(ts *testing.T) {
	ResetForTest()
	host, fsid := "container-1", uint64(0xabc)
	withIdentityHooks(ts, &host, &fsid, nil)
	var warnings []string
	SetWarningHandler(func(m string) { warnings = append(warnings, m) })
	dir := testExeRoot(ts)
	path := filepath.Join(dir, "weak.json")
	err := LoadConfig(&TestConfig{DatabasePassword: "x"}, 1, path, false, false)
	if err == nil || !strings.Contains(err.Error(), t("config.weak_identity")) {
		ts.Fatalf("expected the weak identity error, got %v", err)
	}
	SetAllowWeakIdentity(true)
	ResetForTest()
	if err := LoadConfig(&TestConfig{DatabasePassword: "x"}, 1, path, false, false); err != nil {
		ts.Fatalf("weak identity must work after opt-in: %v", err)
	}
	if err := UpdateConfig(&TestConfig{DatabasePassword: "y"}, path); err != nil {
		ts.Fatal(err)
	}
	if len(warnings) != 1 || warnings[0] != t("config.warn_weak_identity") {
		ts.Errorf("expected exactly one weak identity warning, got %q", warnings)
	}
	// a key file is a strong source
	SetAllowWeakIdentity(false)
	if err := SetKeyFile(filepath.Join(dir, "weak.key")); err != nil {
		ts.Fatal(err)
	}
	ResetForTest()
	if err := LoadConfig(&TestConfig{DatabasePassword: "z"}, 1, filepath.Join(dir, "keyfile.json"), false, false); err != nil {
		ts.Fatalf("a key file must count as strong source: %v", err)
	}
}

// TestExcludedSourcesAreRemoved checks SA-25: SetUseHostname(false) and
// SetUseFileSystemID(false) re-encrypt existing values without them.
func TestExcludedSourcesAreRemoved(ts *testing.T) {
	ResetForTest()
	host, fsid := "host-a", uint64(0xaaa)
	withIdentityHooks(ts, &host, &fsid, map[string]string{sourceMachineID: "0123456789abcdef0123456789abcdef"})
	dir := testExeRoot(ts)
	path := filepath.Join(dir, "exclude.json")
	if err := LoadConfig(&TestConfig{DatabasePassword: "portable"}, 1, path, false, false); err != nil {
		ts.Fatal(err)
	}
	raw, _ := os.ReadFile(path)
	if !strings.Contains(string(raw), "v3:fsid,hostname,machineid,") {
		ts.Fatalf("expected fsid and hostname in the value: %s", raw)
	}
	SetUseHostname(false)
	SetUseFileSystemID(false)
	ResetForTest()
	if err := LoadConfig(&TestConfig{}, 1, path, false, false); err != nil {
		ts.Fatal(err)
	}
	raw, _ = os.ReadFile(path)
	if !strings.Contains(string(raw), `"v3:machineid,machtype:`) {
		ts.Fatalf("hostname and fsid must be removed: %s", raw)
	}
	// now portable: other host name and file system
	host, fsid = "host-b", 0xbbb
	ResetForTest()
	cfg := &TestConfig{}
	if err := LoadConfig(cfg, 1, path, false, false); err != nil || cfg.DatabasePassword != "portable" {
		ts.Fatalf("value must be portable: %q %v", cfg.DatabasePassword, err)
	}
}

// TestSourceAllowlist checks SA-19.
func TestSourceAllowlist(ts *testing.T) {
	for _, v := range []string{"v2:hostname:QUJD", "v2:fsid:QUJD", "v2:unknown:QUJD"} {
		if _, _, err := parseV2Value(v); !errors.Is(err, errInvalidCiphertext) {
			ts.Errorf("%q: v2 must reject the source, got %v", v, err)
		}
	}
	if _, _, err := parseV3Value("v3:hostname,unknown:QUJD"); !errors.Is(err, errInvalidCiphertext) {
		ts.Errorf("v3 must reject unknown sources, got %v", err)
	}
}

// TestMaskIdentityValue checks SA-24.
func TestMaskIdentityValue(ts *testing.T) {
	masked := maskIdentityValue("build-01.example.com")
	if strings.Contains(masked, "example") || !strings.HasPrefix(masked, "bu") {
		ts.Errorf("unexpected mask %q", masked)
	}
	if maskIdentityValue("build-01.example.com") != masked || maskIdentityValue("other") == masked {
		ts.Error("mask must be deterministic and distinguish values")
	}
}
