package config

import (
	"strings"
	"testing"
)

func TestGetEnv(t *testing.T) {
	// Test with set environment variable
	t.Setenv("TEST_ENV_VAR", "test_value")

	value := getEnv("TEST_ENV_VAR", "default")
	if value != "test_value" {
		t.Errorf("getEnv() = %v, want %v", value, "test_value")
	}

	// Test with unset environment variable
	value = getEnv("UNSET_TEST_VAR", "default")
	if value != "default" {
		t.Errorf("getEnv() = %v, want %v", value, "default")
	}
}

func TestGetEnvAsInt(t *testing.T) {
	// Test with valid integer
	t.Setenv("TEST_INT_VAR", "42")

	value := getEnvAsInt("TEST_INT_VAR", 0)
	if value != 42 {
		t.Errorf("getEnvAsInt() = %v, want %v", value, 42)
	}

	// Test with unset variable
	value = getEnvAsInt("UNSET_INT_VAR", 100)
	if value != 100 {
		t.Errorf("getEnvAsInt() = %v, want %v", value, 100)
	}

	// Test with invalid integer
	t.Setenv("TEST_INVALID_INT", "not_a_number")

	value = getEnvAsInt("TEST_INVALID_INT", 50)
	if value != 50 {
		t.Errorf("getEnvAsInt() = %v, want %v", value, 50)
	}
}

func TestGetEnvAsBool(t *testing.T) {
	tests := []struct {
		name         string
		value        string
		defaultValue bool
		want         bool
	}{
		{"true value", "true", false, true},
		{"false value", "false", true, false},
		{"1 as true", "1", false, true},
		{"0 as false", "0", true, false},
		{"unset uses default", "", true, true},
		{"invalid uses default", "invalid", false, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("TEST_BOOL_VAR", tt.value)
			got := getEnvAsBool("TEST_BOOL_VAR", tt.defaultValue)
			if got != tt.want {
				t.Errorf("getEnvAsBool() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestLoad(t *testing.T) {
	envKeys := []string{
		"GHOSTMAIL_SMTP_HOST", "GHOSTMAIL_SMTP_PORT", "GHOSTMAIL_SMTP_USERNAME",
		"GHOSTMAIL_SMTP_PASSWORD", "GHOSTMAIL_SMTP_FROM", "GHOSTMAIL_SMTP_USE_TLS", "GHOSTMAIL_SMTP_STARTTLS", "GHOSTMAIL_SMTP_ALLOW_INSECURE",
		"GHOSTMAIL_IMAP_HOST", "GHOSTMAIL_IMAP_PORT", "GHOSTMAIL_IMAP_USERNAME",
		"GHOSTMAIL_IMAP_PASSWORD", "GHOSTMAIL_IMAP_MAILBOX", "GHOSTMAIL_IMAP_USE_TLS", "GHOSTMAIL_IMAP_STARTTLS", "GHOSTMAIL_IMAP_ALLOW_INSECURE",
	}
	for _, key := range envKeys {
		t.Setenv(key, "")
	}

	// Set test values
	t.Setenv("GHOSTMAIL_SMTP_HOST", "smtp.example.com")
	t.Setenv("GHOSTMAIL_SMTP_USERNAME", "test@example.com")
	t.Setenv("GHOSTMAIL_SMTP_PASSWORD", "testpass")
	t.Setenv("GHOSTMAIL_IMAP_HOST", "imap.example.com")
	t.Setenv("GHOSTMAIL_IMAP_USERNAME", "test@example.com")
	t.Setenv("GHOSTMAIL_IMAP_PASSWORD", "testpass")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	// Check SMTP config
	if cfg.SMTP.Host != "smtp.example.com" {
		t.Errorf("SMTP.Host = %v, want %v", cfg.SMTP.Host, "smtp.example.com")
	}
	if cfg.SMTP.Port != 587 {
		t.Errorf("SMTP.Port = %v, want %v", cfg.SMTP.Port, 587)
	}
	if cfg.SMTP.Username != "test@example.com" {
		t.Errorf("SMTP.Username = %v, want %v", cfg.SMTP.Username, "test@example.com")
	}

	// Check IMAP config
	if cfg.IMAP.Host != "imap.example.com" {
		t.Errorf("IMAP.Host = %v, want %v", cfg.IMAP.Host, "imap.example.com")
	}
	if cfg.IMAP.Port != 993 {
		t.Errorf("IMAP.Port = %v, want %v", cfg.IMAP.Port, 993)
	}
}

func TestLoadOverridesAndInvalidValues(t *testing.T) {
	t.Setenv("GHOSTMAIL_SMTP_PORT", "2525")
	t.Setenv("GHOSTMAIL_SMTP_USE_TLS", "true")
	t.Setenv("GHOSTMAIL_SMTP_STARTTLS", "false")
	t.Setenv("GHOSTMAIL_IMAP_PORT", "not-a-port")
	t.Setenv("GHOSTMAIL_IMAP_USE_TLS", "not-a-bool")
	t.Setenv("GHOSTMAIL_IMAP_MAILBOX", "Archive")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.SMTP.Port != 2525 || !cfg.SMTP.UseTLS || cfg.SMTP.StartTLS {
		t.Errorf("SMTP = %+v, want overridden port and TLS values", cfg.SMTP)
	}
	if cfg.IMAP.Port != 993 || !cfg.IMAP.UseTLS || cfg.IMAP.Mailbox != "Archive" {
		t.Errorf("IMAP = %+v, want defaults for invalid values and overridden mailbox", cfg.IMAP)
	}
}

func TestValidateSMTP(t *testing.T) {
	tests := []struct {
		name    string
		config  Config
		wantErr bool
	}{
		{
			name: "valid config",
			config: Config{
				SMTP: SMTPConfig{
					Host:     "smtp.example.com",
					Port:     587,
					Username: "test@example.com",
					Password: "password",
					StartTLS: true,
				},
			},
			wantErr: false,
		},
		{
			name: "plaintext requires explicit opt-in",
			config: Config{SMTP: SMTPConfig{
				Host: "smtp.example.com", Port: 587, Username: "test@example.com", Password: "password",
			}},
			wantErr: true,
		},
		{
			name: "explicit plaintext opt-in",
			config: Config{SMTP: SMTPConfig{
				Host: "smtp.example.com", Port: 587, Username: "test@example.com", Password: "password", AllowInsecure: true,
			}},
			wantErr: false,
		},
		{
			name: "port below range",
			config: Config{SMTP: SMTPConfig{
				Host: "smtp.example.com", Port: 0, Username: "test@example.com", Password: "password", StartTLS: true,
			}},
			wantErr: true,
		},
		{
			name: "port above range",
			config: Config{SMTP: SMTPConfig{
				Host: "smtp.example.com", Port: 70000, Username: "test@example.com", Password: "password", StartTLS: true,
			}},
			wantErr: true,
		},
		{
			name: "missing host",
			config: Config{
				SMTP: SMTPConfig{
					Host:     "",
					Username: "test@example.com",
					Password: "password",
				},
			},
			wantErr: true,
		},
		{
			name: "missing username",
			config: Config{
				SMTP: SMTPConfig{
					Host:     "smtp.example.com",
					Username: "",
					Password: "password",
				},
			},
			wantErr: true,
		},
		{
			name: "missing password",
			config: Config{
				SMTP: SMTPConfig{
					Host:     "smtp.example.com",
					Username: "test@example.com",
					Password: "",
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.ValidateSMTP()
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateSMTP() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr && !strings.Contains(err.Error(), "GHOSTMAIL_SMTP_") {
				t.Errorf("ValidateSMTP() error = %q, want environment variable name", err)
			}
		})
	}
}

func TestValidateIMAP(t *testing.T) {
	tests := []struct {
		name    string
		config  Config
		wantErr bool
	}{
		{
			name: "valid config",
			config: Config{
				IMAP: IMAPConfig{
					Host:     "imap.example.com",
					Port:     993,
					Username: "test@example.com",
					Password: "password",
					UseTLS:   true,
				},
			},
			wantErr: false,
		},
		{
			name: "plaintext requires explicit opt-in",
			config: Config{IMAP: IMAPConfig{
				Host: "imap.example.com", Port: 993, Username: "test@example.com", Password: "password",
			}},
			wantErr: true,
		},
		{
			name: "STARTTLS is secure",
			config: Config{IMAP: IMAPConfig{
				Host: "imap.example.com", Port: 143, Username: "test@example.com", Password: "password", StartTLS: true,
			}},
			wantErr: false,
		},
		{
			name: "port below range",
			config: Config{IMAP: IMAPConfig{
				Host: "imap.example.com", Port: 0, Username: "test@example.com", Password: "password", UseTLS: true,
			}},
			wantErr: true,
		},
		{
			name: "port above range",
			config: Config{IMAP: IMAPConfig{
				Host: "imap.example.com", Port: 70000, Username: "test@example.com", Password: "password", UseTLS: true,
			}},
			wantErr: true,
		},
		{
			name: "missing host",
			config: Config{
				IMAP: IMAPConfig{
					Host:     "",
					Username: "test@example.com",
					Password: "password",
				},
			},
			wantErr: true,
		},
		{
			name: "missing username",
			config: Config{
				IMAP: IMAPConfig{
					Host:     "imap.example.com",
					Username: "",
					Password: "password",
				},
			},
			wantErr: true,
		},
		{
			name: "missing password",
			config: Config{
				IMAP: IMAPConfig{
					Host:     "imap.example.com",
					Username: "test@example.com",
					Password: "",
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.ValidateIMAP()
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateIMAP() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr && !strings.Contains(err.Error(), "GHOSTMAIL_IMAP_") {
				t.Errorf("ValidateIMAP() error = %q, want environment variable name", err)
			}
		})
	}
}
