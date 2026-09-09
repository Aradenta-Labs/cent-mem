package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/aradenta-labs/cent-mem/internal/config"
)

func TestConfigDefaults(t *testing.T) {
	os.Clearenv()

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		homeDir = "."
	}
	expectedHome := filepath.Join(homeDir, ".centmem")

	if cfg.Home != expectedHome {
		t.Errorf("expected home %q, got %q", expectedHome, cfg.Home)
	}

	expectedDB := filepath.Join(expectedHome, "centmem.db")
	if cfg.DBPath != expectedDB {
		t.Errorf("expected DBPath %q, got %q", expectedDB, cfg.DBPath)
	}

	if cfg.Model.Name != "bge-small-en-v1.5" {
		t.Errorf("expected model name %q, got %q", "bge-small-en-v1.5", cfg.Model.Name)
	}

	expectedModelPath := filepath.Join(expectedHome, "models", "bge-small-en-v1.5.onnx")
	if cfg.Model.Path != expectedModelPath {
		t.Errorf("expected model path %q, got %q", expectedModelPath, cfg.Model.Path)
	}

	if cfg.Model.Dims != 384 {
		t.Errorf("expected dimensions 384, got %d", cfg.Model.Dims)
	}

	expectedSock := filepath.Join(expectedHome, "centmemd.sock")
	if cfg.Daemon.SocketPath != expectedSock {
		t.Errorf("expected socket path %q, got %q", expectedSock, cfg.Daemon.SocketPath)
	}
	expectedPID := filepath.Join(expectedHome, "centmemd.pid")
	if cfg.Daemon.PIDPath != expectedPID {
		t.Errorf("expected pid path %q, got %q", expectedPID, cfg.Daemon.PIDPath)
	}
	if cfg.Daemon.Port != 0 {
		t.Errorf("expected daemon port 0, got %d", cfg.Daemon.Port)
	}
}

func TestConfigEnvOverride(t *testing.T) {
	os.Clearenv()
	os.Setenv("CENTMEM_HOME", "/tmp/custom_home")
	os.Setenv("CENTMEM_DB", "/tmp/custom_db.sqlite")
	os.Setenv("CENTMEM_MODEL", "custom_model")
	os.Setenv("CENTMEM_MODEL_DIMS", "768")
	os.Setenv("CENTMEM_DAEMON_SOCKET", "/tmp/custom.sock")
	os.Setenv("CENTMEM_DAEMON_PID", "/tmp/custom.pid")
	os.Setenv("CENTMEM_DAEMON_PORT", "50051")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	if cfg.Home != "/tmp/custom_home" {
		t.Errorf("expected custom home, got %q", cfg.Home)
	}

	if cfg.DBPath != "/tmp/custom_db.sqlite" {
		t.Errorf("expected custom DB path, got %q", cfg.DBPath)
	}

	if cfg.Model.Name != "custom_model" {
		t.Errorf("expected custom model name, got %q", cfg.Model.Name)
	}

	expectedModelPath := filepath.Join("/tmp/custom_home", "models", "custom_model.onnx")
	if cfg.Model.Path != expectedModelPath {
		t.Errorf("expected custom model path, got %q", cfg.Model.Path)
	}

	if cfg.Model.Dims != 768 {
		t.Errorf("expected custom dimensions 768, got %d", cfg.Model.Dims)
	}

	if cfg.Daemon.SocketPath != "/tmp/custom.sock" {
		t.Errorf("expected socket path /tmp/custom.sock, got %q", cfg.Daemon.SocketPath)
	}
	if cfg.Daemon.PIDPath != "/tmp/custom.pid" {
		t.Errorf("expected pid path /tmp/custom.pid, got %q", cfg.Daemon.PIDPath)
	}
	if cfg.Daemon.Port != 50051 {
		t.Errorf("expected daemon port 50051, got %d", cfg.Daemon.Port)
	}
}

func TestConfigEnsure(t *testing.T) {
	tempHome := filepath.Join(t.TempDir(), "centmem_test_home")
	cfg := config.Config{
		Home:   tempHome,
		DBPath: filepath.Join(tempHome, "centmem.db"),
		Model: config.ModelConfig{
			Name: "test-model",
			Path: filepath.Join(tempHome, "models", "test-model.onnx"),
			Dims: 128,
		},
	}

	if err := cfg.Ensure(); err != nil {
		t.Fatalf("failed to ensure config directories: %v", err)
	}

	info, err := os.Stat(tempHome)
	if err != nil {
		t.Fatalf("home dir not created: %v", err)
	}

	if !info.IsDir() {
		t.Fatalf("home is not a directory")
	}

	// Permissions check on UNIX-like filesystems
	mode := info.Mode().Perm()
	if os.PathSeparator == '/' && mode != 0700 {
		t.Errorf("expected permissions 0700 for home, got %o", mode)
	}

	modelDir := filepath.Dir(cfg.Model.Path)
	modelInfo, err := os.Stat(modelDir)
	if err != nil {
		t.Fatalf("models dir not created: %v", err)
	}
	if !modelInfo.IsDir() {
		t.Fatalf("models path is not a directory")
	}
}

func TestConfigEnsureEnforces0700(t *testing.T) {
	if os.PathSeparator != '/' {
		t.Skip("permission enforcement is unix-only")
	}
	tempHome := filepath.Join(t.TempDir(), "precreated")
	// Pre-create the dir with permissive perms to confirm Ensure fixes them.
	if err := os.MkdirAll(tempHome, 0755); err != nil {
		t.Fatal(err)
	}
	cfg := config.Config{
		Home:   tempHome,
		DBPath: filepath.Join(tempHome, "centmem.db"),
		Model:  config.ModelConfig{Name: "m", Path: filepath.Join(tempHome, "models", "m.onnx"), Dims: 1},
	}
	if err := cfg.Ensure(); err != nil {
		t.Fatalf("Ensure: %v", err)
	}
	info, err := os.Stat(tempHome)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != 0700 {
		t.Errorf("home perms = %o, want 0700", got)
	}
}

func TestConfigRetentionDefaults(t *testing.T) {
	os.Clearenv()
	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	r := cfg.Retention
	if r.FactKeepDays != 0 {
		t.Errorf("FactKeepDays = %d, want 0", r.FactKeepDays)
	}
	if r.NoteSummarizeAfterDays != 30 {
		t.Errorf("NoteSummarizeAfterDays = %d, want 30", r.NoteSummarizeAfterDays)
	}
	if r.LogSummarizeAfterDays != 14 {
		t.Errorf("LogSummarizeAfterDays = %d, want 14", r.LogSummarizeAfterDays)
	}
	if r.LogDropAfterDays != 30 {
		t.Errorf("LogDropAfterDays = %d, want 30", r.LogDropAfterDays)
	}
	if r.ArchiveKeepDays != 365 {
		t.Errorf("ArchiveKeepDays = %d, want 365", r.ArchiveKeepDays)
	}
}

func TestConfigRetentionEnvOverride(t *testing.T) {
	os.Clearenv()
	os.Setenv("CENTMEM_RETENTION_NOTE_SUMMARIZE_AFTER_DAYS", "60")
	os.Setenv("CENTMEM_RETENTION_LOG_SUMMARIZE_AFTER_DAYS", "21")
	os.Setenv("CENTMEM_RETENTION_FACT_KEEP_DAYS", "0")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if cfg.Retention.NoteSummarizeAfterDays != 60 {
		t.Errorf("NoteSummarizeAfterDays = %d, want 60", cfg.Retention.NoteSummarizeAfterDays)
	}
	if cfg.Retention.LogSummarizeAfterDays != 21 {
		t.Errorf("LogSummarizeAfterDays = %d, want 21", cfg.Retention.LogSummarizeAfterDays)
	}
	if cfg.Retention.FactKeepDays != 0 {
		t.Errorf("FactKeepDays = %d, want 0", cfg.Retention.FactKeepDays)
	}
}

func TestConfigRetentionRejectsNegative(t *testing.T) {
	os.Clearenv()
	os.Setenv("CENTMEM_RETENTION_NOTE_SUMMARIZE_AFTER_DAYS", "-5")
	if _, err := config.Load(); err == nil {
		t.Fatal("expected error for negative note_summarize_after_days")
	}
}

func TestConfigDefaultRetention(t *testing.T) {
	r := config.DefaultRetention()
	if r.NoteSummarizeAfterDays != 30 || r.LogSummarizeAfterDays != 14 {
		t.Errorf("unexpected defaults: %+v", r)
	}
}

func TestConfigCaptureEnvOverrides(t *testing.T) {
	os.Clearenv()
	os.Setenv("CENTMEM_CAPTURE_ENABLED", "true")
	os.Setenv("CENTMEM_CAPTURE_HARNESS", "claude-code")
	os.Setenv("CENTMEM_CAPTURE_SCOPE", "project:test")
	os.Setenv("CENTMEM_CAPTURE_BACKEND", "openai-compatible")
	os.Setenv("CENTMEM_CAPTURE_API_BASE_URL", "https://api.custom.com/v1")
	os.Setenv("CENTMEM_CAPTURE_API_KEY_ENV", "CUSTOM_API_KEY")
	os.Setenv("CENTMEM_CAPTURE_API_MODEL", "gpt-4o")
	os.Setenv("CENTMEM_CAPTURE_CONFIDENCE_THRESHOLD", "0.85")
	os.Setenv("CENTMEM_CAPTURE_CATEGORIES", "decision,fact,custom_cat")
	os.Setenv("CENTMEM_CAPTURE_TRIGGERS", "session-end,on-demand")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	if !cfg.Capture.Enabled {
		t.Errorf("expected Enabled to be true")
	}
	if cfg.Capture.Harness != "claude-code" {
		t.Errorf("expected Harness claude-code, got %q", cfg.Capture.Harness)
	}
	if cfg.Capture.Scope != "project:test" {
		t.Errorf("expected Scope project:test, got %q", cfg.Capture.Scope)
	}
	if cfg.Capture.Backend != "openai-compatible" {
		t.Errorf("expected Backend openai-compatible, got %q", cfg.Capture.Backend)
	}
	if cfg.Capture.APIBaseURL != "https://api.custom.com/v1" {
		t.Errorf("expected APIBaseURL https://api.custom.com/v1, got %q", cfg.Capture.APIBaseURL)
	}
	if cfg.Capture.APIKeyEnv != "CUSTOM_API_KEY" {
		t.Errorf("expected APIKeyEnv CUSTOM_API_KEY, got %q", cfg.Capture.APIKeyEnv)
	}
	if cfg.Capture.APIModel != "gpt-4o" {
		t.Errorf("expected APIModel gpt-4o, got %q", cfg.Capture.APIModel)
	}
	if cfg.Capture.ConfidenceThreshold != 0.85 {
		t.Errorf("expected ConfidenceThreshold 0.85, got %f", cfg.Capture.ConfidenceThreshold)
	}
	if len(cfg.Capture.Categories) != 3 || cfg.Capture.Categories[2] != "custom_cat" {
		t.Errorf("unexpected categories: %+v", cfg.Capture.Categories)
	}
	if len(cfg.Capture.Triggers) != 2 || cfg.Capture.Triggers[0] != "session-end" {
		t.Errorf("unexpected triggers: %+v", cfg.Capture.Triggers)
	}
}

func TestConfigCaptureRejectsInvalidBackend(t *testing.T) {
	os.Clearenv()
	t.Setenv("CENTMEM_CAPTURE_BACKEND", "invalid-backend")
	if _, err := config.Load(); err == nil {
		t.Fatal("expected error for invalid capture backend")
	}
}

func TestResolveAPIKey(t *testing.T) {
	// 1. Unset env var
	os.Unsetenv("TEST_UNSET_API_KEY_VAR")
	key, fromEnv := config.ResolveAPIKey("TEST_UNSET_API_KEY_VAR")
	if key != "" || !fromEnv {
		t.Errorf("expected empty key and fromEnv=true for unset var, got key=%q, fromEnv=%v", key, fromEnv)
	}

	// 2. Set env var
	t.Setenv("TEST_SET_API_KEY_VAR", "sk-resolved-from-env")
	key, fromEnv = config.ResolveAPIKey("TEST_SET_API_KEY_VAR")
	if key != "sk-resolved-from-env" || !fromEnv {
		t.Errorf("expected sk-resolved-from-env and fromEnv=true, got key=%q, fromEnv=%v", key, fromEnv)
	}

	// 3. Direct API key like in User's screenshot: sk-f7c1cf87505b4556-yi1kjc-2435b0d4
	userKey := "sk-f7c1cf87505b4556-yi1kjc-2435b0d4"
	key, fromEnv = config.ResolveAPIKey(userKey)
	if key != userKey || fromEnv {
		t.Errorf("expected userKey direct resolution and fromEnv=false, got key=%q, fromEnv=%v", key, fromEnv)
	}

	// 4. Direct API key with Bearer prefix
	key, fromEnv = config.ResolveAPIKey("Bearer sk-direct-with-bearer-12345")
	if key != "sk-direct-with-bearer-12345" || fromEnv {
		t.Errorf("expected stripped bearer key and fromEnv=false, got key=%q, fromEnv=%v", key, fromEnv)
	}

	// 5. Empty input
	key, fromEnv = config.ResolveAPIKey("")
	if key != "" || fromEnv {
		t.Errorf("expected empty key and fromEnv=false for empty input, got key=%q, fromEnv=%v", key, fromEnv)
	}

	// 6. Direct key with gsk_ prefix
	key, fromEnv = config.ResolveAPIKey("gsk_some_groq_key_value")
	if key != "gsk_some_groq_key_value" || fromEnv {
		t.Errorf("expected gsk key direct resolution, got key=%q, fromEnv=%v", key, fromEnv)
	}

	// 7. Direct key with 32-char hex string
	hexKey := "a1b2c3d4e5f60718293a4b5c6d7e8f90"
	key, fromEnv = config.ResolveAPIKey(hexKey)
	if key != hexKey || fromEnv {
		t.Errorf("expected hexKey direct resolution, got key=%q, fromEnv=%v", key, fromEnv)
	}

	// 8. Lowercase bearer prefix and whitespace
	key, fromEnv = config.ResolveAPIKey("  bearer sk-lowercase-bearer-12345  ")
	if key != "sk-lowercase-bearer-12345" || fromEnv {
		t.Errorf("expected stripped lowercase bearer key, got key=%q, fromEnv=%v", key, fromEnv)
	}

	// 9. Surrounding double and single quotes
	key, fromEnv = config.ResolveAPIKey(`"sk-quoted-token-12345"`)
	if key != "sk-quoted-token-12345" || fromEnv {
		t.Errorf("expected stripped double quotes, got key=%q, fromEnv=%v", key, fromEnv)
	}
	key, fromEnv = config.ResolveAPIKey(`'sk-single-quoted-12345'`)
	if key != "sk-single-quoted-12345" || fromEnv {
		t.Errorf("expected stripped single quotes, got key=%q, fromEnv=%v", key, fromEnv)
	}

	// 10. Nested Bearer and quotes
	key, fromEnv = config.ResolveAPIKey(`Bearer "sk-nested-bearer-quotes-12345"`)
	if key != "sk-nested-bearer-quotes-12345" || fromEnv {
		t.Errorf("expected stripped nested bearer and quotes, got key=%q, fromEnv=%v", key, fromEnv)
	}

	// 11. Short lowercase direct key
	key, fromEnv = config.ResolveAPIKey("secret")
	if key != "secret" || fromEnv {
		t.Errorf("expected secret direct resolution, got key=%q, fromEnv=%v", key, fromEnv)
	}

	// 12. sk_live_ token
	key, fromEnv = config.ResolveAPIKey("sk_live_1234567890abcdef")
	if key != "sk_live_1234567890abcdef" || fromEnv {
		t.Errorf("expected sk_live direct resolution, got key=%q, fromEnv=%v", key, fromEnv)
	}
}

func TestCaptureConfig_TOML_APIKey(t *testing.T) {
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "config.toml")

	// 1. Save and Load with APIKey set
	cfg := config.Config{
		Model:     config.ModelConfig{Name: "bge-small-en-v1.5", Dims: 384},
		Retention: config.DefaultRetention(),
		Capture:   config.DefaultCaptureConfig(),
		Search:    config.DefaultSearchConfig(),
	}
	cfg.Capture.APIKey = "sk-direct-toml-key"
	cfg.Capture.APIKeyEnv = "sk-direct-toml-key"

	if err := config.SaveTOML(cfgPath, cfg); err != nil {
		t.Fatalf("SaveTOML failed: %v", err)
	}

	loaded, err := config.LoadTOML(cfgPath)
	if err != nil {
		t.Fatalf("LoadTOML failed: %v", err)
	}
	if loaded.Capture.APIKey != "sk-direct-toml-key" {
		t.Errorf("expected Capture.APIKey to round-trip in TOML, got %q", loaded.Capture.APIKey)
	}
	if loaded.Capture.APIKeyEnv != "sk-direct-toml-key" {
		t.Errorf("expected Capture.APIKeyEnv to round-trip in TOML, got %q", loaded.Capture.APIKeyEnv)
	}
}



func TestConfigSearch_Importance_EnvAndValidation(t *testing.T) {
	home := t.TempDir()
	t.Setenv("CENTMEM_HOME", home)

	// Verify defaults
	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !cfg.Search.ImportanceBoostEnabled {
		t.Errorf("expected ImportanceBoostEnabled default true")
	}
	if cfg.Search.ImportanceWeight != 0.1 {
		t.Errorf("expected ImportanceWeight default 0.1, got %f", cfg.Search.ImportanceWeight)
	}
	if cfg.Search.ImportanceCap != 2.0 {
		t.Errorf("expected ImportanceCap default 2.0, got %f", cfg.Search.ImportanceCap)
	}

	// Env overrides
	t.Setenv("CENTMEM_SEARCH_IMPORTANCE_BOOST_ENABLED", "false")
	t.Setenv("CENTMEM_SEARCH_IMPORTANCE_WEIGHT", "0.25")
	t.Setenv("CENTMEM_SEARCH_IMPORTANCE_CAP", "3.5")

	cfg, err = config.Load()
	if err != nil {
		t.Fatalf("Load with env overrides: %v", err)
	}
	if cfg.Search.ImportanceBoostEnabled {
		t.Errorf("expected ImportanceBoostEnabled=false from env")
	}
	if cfg.Search.ImportanceWeight != 0.25 {
		t.Errorf("expected ImportanceWeight=0.25 from env, got %f", cfg.Search.ImportanceWeight)
	}
	if cfg.Search.ImportanceCap != 3.5 {
		t.Errorf("expected ImportanceCap=3.5 from env, got %f", cfg.Search.ImportanceCap)
	}

	// Validation rejection: cap < 1.0 including 0.0
	t.Setenv("CENTMEM_SEARCH_IMPORTANCE_CAP", "0.5")
	_, err = config.Load()
	if err == nil {
		t.Errorf("expected error for importance_cap = 0.5, got nil")
	}

	t.Setenv("CENTMEM_SEARCH_IMPORTANCE_CAP", "0.0")
	_, err = config.Load()
	if err == nil {
		t.Errorf("expected error for importance_cap = 0.0, got nil")
	}

	// TOML zero weight preservation
	t.Setenv("CENTMEM_SEARCH_IMPORTANCE_CAP", "")
	t.Setenv("CENTMEM_SEARCH_IMPORTANCE_WEIGHT", "")
	t.Setenv("CENTMEM_SEARCH_IMPORTANCE_BOOST_ENABLED", "")
	tomlPath := filepath.Join(home, "config.toml")
	if err := os.WriteFile(tomlPath, []byte("[search]\nimportance_weight = 0.0\n"), 0644); err != nil {
		t.Fatalf("write toml: %v", err)
	}
	cfgTOML, err := config.Load()
	if err != nil {
		t.Fatalf("Load with toml: %v", err)
	}
	if cfgTOML.Search.ImportanceWeight != 0.0 {
		t.Errorf("expected ImportanceWeight = 0.0 preserved from TOML, got %f", cfgTOML.Search.ImportanceWeight)
	}
	if cfgTOML.Search.ImportanceCap != 2.0 {
		t.Errorf("expected default ImportanceCap = 2.0 preserved from TOML, got %f", cfgTOML.Search.ImportanceCap)
	}
}

func TestConfig_LLMAndAgent(t *testing.T) {
	home := t.TempDir()
	t.Setenv("CENTMEM_HOME", home)

	// 1. Defaults
	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load defaults: %v", err)
	}
	if cfg.LLM.Backend != "ollama" {
		t.Errorf("expected LLM.Backend default 'ollama', got %q", cfg.LLM.Backend)
	}
	if cfg.LLM.Endpoint != "http://127.0.0.1:11434/v1" {
		t.Errorf("expected LLM.Endpoint default 'http://127.0.0.1:11434/v1', got %q", cfg.LLM.Endpoint)
	}
	if cfg.LLM.Model != "deepseek-r1:8b" {
		t.Errorf("expected LLM.Model default 'deepseek-r1:8b', got %q", cfg.LLM.Model)
	}
	if cfg.LLM.TimeoutSeconds != 60 {
		t.Errorf("expected LLM.TimeoutSeconds default 60, got %d", cfg.LLM.TimeoutSeconds)
	}
	if cfg.LLM.MaxTokens != 4096 {
		t.Errorf("expected LLM.MaxTokens default 4096, got %d", cfg.LLM.MaxTokens)
	}
	if cfg.LLM.Temperature != 0.2 {
		t.Errorf("expected LLM.Temperature default 0.2, got %f", cfg.LLM.Temperature)
	}
	if !cfg.Agent.Enabled {
		t.Errorf("expected Agent.Enabled default true")
	}
	if cfg.Agent.MaxReasoningSteps != 8 {
		t.Errorf("expected Agent.MaxReasoningSteps default 8, got %d", cfg.Agent.MaxReasoningSteps)
	}
	if cfg.Agent.ConfidenceThreshold != 0.75 {
		t.Errorf("expected Agent.ConfidenceThreshold default 0.75, got %f", cfg.Agent.ConfidenceThreshold)
	}
	if cfg.Agent.AutoApplySafeLinks {
		t.Errorf("expected Agent.AutoApplySafeLinks default false")
	}

	// 2. Env overrides
	t.Setenv("CENTMEM_LLM_BACKEND", "openai_compatible")
	t.Setenv("CENTMEM_LLM_ENDPOINT", "https://api.openai.com/v1")
	t.Setenv("CENTMEM_LLM_MODEL", "gpt-4o")
	t.Setenv("CENTMEM_LLM_API_KEY", "sk-test-llm-key")
	t.Setenv("CENTMEM_LLM_TIMEOUT_SECONDS", "45")
	t.Setenv("CENTMEM_LLM_MAX_TOKENS", "2048")
	t.Setenv("CENTMEM_LLM_TEMPERATURE", "0.5")

	t.Setenv("CENTMEM_AGENT_ENABLED", "false")
	t.Setenv("CENTMEM_AGENT_MAX_REASONING_STEPS", "12")
	t.Setenv("CENTMEM_AGENT_CONFIDENCE_THRESHOLD", "0.85")
	t.Setenv("CENTMEM_AGENT_AUTO_APPLY_SAFE_LINKS", "true")

	cfg, err = config.Load()
	if err != nil {
		t.Fatalf("Load with env overrides: %v", err)
	}
	if cfg.LLM.Backend != "openai_compatible" || cfg.LLM.Endpoint != "https://api.openai.com/v1" ||
		cfg.LLM.Model != "gpt-4o" || cfg.LLM.APIKey != "sk-test-llm-key" ||
		cfg.LLM.TimeoutSeconds != 45 || cfg.LLM.MaxTokens != 2048 || cfg.LLM.Temperature != 0.5 {
		t.Errorf("LLM env overrides mismatch: %+v", cfg.LLM)
	}
	if cfg.Agent.Enabled || cfg.Agent.MaxReasoningSteps != 12 ||
		cfg.Agent.ConfidenceThreshold != 0.85 || !cfg.Agent.AutoApplySafeLinks {
		t.Errorf("Agent env overrides mismatch: %+v", cfg.Agent)
	}

	// 3. Dot-notation keys: GetConfigValue and SetConfigValue
	val, err := config.GetConfigValue(cfg, "llm.backend")
	if err != nil || val != "openai_compatible" {
		t.Errorf("GetConfigValue llm.backend: %v, val: %v", err, val)
	}
	val, err = config.GetConfigValue(cfg, "agent.max_reasoning_steps")
	if err != nil || val != 12 {
		t.Errorf("GetConfigValue agent.max_reasoning_steps: %v, val: %v", err, val)
	}

	if err := config.SetConfigValue(&cfg, "llm.model", "claude-3-5-sonnet"); err != nil {
		t.Fatalf("SetConfigValue llm.model: %v", err)
	}
	if cfg.LLM.Model != "claude-3-5-sonnet" {
		t.Errorf("expected updated LLM.Model, got %q", cfg.LLM.Model)
	}
	if err := config.SetConfigValue(&cfg, "agent.auto_apply_safe_links", "false"); err != nil {
		t.Fatalf("SetConfigValue agent.auto_apply_safe_links: %v", err)
	}
	if cfg.Agent.AutoApplySafeLinks {
		t.Errorf("expected Agent.AutoApplySafeLinks false")
	}

	// 4. Validation errors
	if err := config.SetConfigValue(&cfg, "llm.backend", "unsupported-backend"); err == nil {
		t.Errorf("expected error for unsupported llm.backend, got nil")
	}
	if err := config.SetConfigValue(&cfg, "agent.confidence_threshold", "1.5"); err == nil {
		t.Errorf("expected error for invalid confidence threshold, got nil")
	}

	// 5. TOML round-trip
	cfgPath := filepath.Join(home, "config.toml")
	if err := config.SaveTOML(cfgPath, cfg); err != nil {
		t.Fatalf("SaveTOML failed: %v", err)
	}
	loaded, err := config.LoadTOML(cfgPath)
	if err != nil {
		t.Fatalf("LoadTOML failed: %v", err)
	}
	if loaded.LLM.Model != "claude-3-5-sonnet" {
		t.Errorf("expected loaded.LLM.Model to be claude-3-5-sonnet, got %q", loaded.LLM.Model)
	}
}

