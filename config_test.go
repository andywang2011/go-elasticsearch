package elasticsearch

import "testing"

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.Timeout <= 0 {
		t.Error("expected positive timeout")
	}
	if cfg.RefreshPolicy != RefreshFalse {
		t.Errorf("RefreshPolicy = %q", cfg.RefreshPolicy)
	}
}

func TestConfig_Validate(t *testing.T) {
	cfg := Config{Addresses: []string{"http://localhost:9200"}}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate() = %v", err)
	}
	if cfg.Timeout <= 0 {
		t.Error("expected default timeout")
	}
}

func TestConfig_Validate_MissingAddresses(t *testing.T) {
	var cfg Config
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected error")
	}
}

func TestConfig_Validate_InvalidRefresh(t *testing.T) {
	cfg := Config{Addresses: []string{"http://localhost:9200"}, RefreshPolicy: "invalid"}
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected invalid refresh policy error")
	}
}
