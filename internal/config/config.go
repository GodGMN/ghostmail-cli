// Package config provides configuration management for ghostmail-cli.
package config

import (
	"fmt"
	"os"
	"strconv"
)

// Config holds all configuration for the application.
type Config struct {
	SMTP SMTPConfig `json:"smtp"`
	IMAP IMAPConfig `json:"imap"`
}

// SMTPConfig holds SMTP server configuration.
type SMTPConfig struct {
	Host          string `json:"host"`
	Port          int    `json:"port"`
	Username      string `json:"username"`
	Password      string `json:"password"`
	UseTLS        bool   `json:"use_tls"`
	StartTLS      bool   `json:"start_tls"`
	AllowInsecure bool   `json:"allow_insecure"`
	From          string `json:"from"`
}

// IMAPConfig holds IMAP server configuration.
type IMAPConfig struct {
	Host          string `json:"host"`
	Port          int    `json:"port"`
	Username      string `json:"username"`
	Password      string `json:"password"`
	UseTLS        bool   `json:"use_tls"`
	StartTLS      bool   `json:"start_tls"`
	AllowInsecure bool   `json:"allow_insecure"`
	Mailbox       string `json:"mailbox"`
}

// Load loads configuration from environment variables.
func Load() (*Config, error) {
	cfg := &Config{
		SMTP: SMTPConfig{
			Host:     getEnv("GHOSTMAIL_SMTP_HOST", ""),
			Port:     getEnvAsInt("GHOSTMAIL_SMTP_PORT", 587),
			Username: getEnv("GHOSTMAIL_SMTP_USERNAME", ""),
			Password: getEnv("GHOSTMAIL_SMTP_PASSWORD", ""),
			UseTLS:   getEnvAsBool("GHOSTMAIL_SMTP_USE_TLS", false),
			StartTLS: getEnvAsBool("GHOSTMAIL_SMTP_STARTTLS", true),
			// GHOSTMAIL_SMTP_ALLOW_INSECURE permits plaintext credentials. Do not use in production.
			AllowInsecure: getEnvAsBool("GHOSTMAIL_SMTP_ALLOW_INSECURE", false),
			From:          getEnv("GHOSTMAIL_SMTP_FROM", ""),
		},
		IMAP: IMAPConfig{
			Host:     getEnv("GHOSTMAIL_IMAP_HOST", ""),
			Port:     getEnvAsInt("GHOSTMAIL_IMAP_PORT", 993),
			Username: getEnv("GHOSTMAIL_IMAP_USERNAME", ""),
			Password: getEnv("GHOSTMAIL_IMAP_PASSWORD", ""),
			UseTLS:   getEnvAsBool("GHOSTMAIL_IMAP_USE_TLS", true),
			StartTLS: getEnvAsBool("GHOSTMAIL_IMAP_STARTTLS", false),
			// GHOSTMAIL_IMAP_ALLOW_INSECURE permits plaintext credentials. Do not use in production.
			AllowInsecure: getEnvAsBool("GHOSTMAIL_IMAP_ALLOW_INSECURE", false),
			Mailbox:       getEnv("GHOSTMAIL_IMAP_MAILBOX", "INBOX"),
		},
	}

	return cfg, nil
}

// ValidateSMTP validates SMTP configuration.
func (c *Config) ValidateSMTP() error {
	if c.SMTP.Host == "" {
		return fmt.Errorf("SMTP host is required (set GHOSTMAIL_SMTP_HOST)")
	}
	if c.SMTP.Username == "" {
		return fmt.Errorf("SMTP username is required (set GHOSTMAIL_SMTP_USERNAME)")
	}
	if c.SMTP.Password == "" {
		return fmt.Errorf("SMTP password is required (set GHOSTMAIL_SMTP_PASSWORD)")
	}
	if !c.SMTP.UseTLS && !c.SMTP.StartTLS && !c.SMTP.AllowInsecure {
		return fmt.Errorf("SMTP requires TLS or STARTTLS (set GHOSTMAIL_SMTP_USE_TLS=true or GHOSTMAIL_SMTP_STARTTLS=true; GHOSTMAIL_SMTP_ALLOW_INSECURE=true permits plaintext credentials and is unsafe)")
	}
	return nil
}

// ValidateIMAP validates IMAP configuration.
func (c *Config) ValidateIMAP() error {
	if c.IMAP.Host == "" {
		return fmt.Errorf("IMAP host is required (set GHOSTMAIL_IMAP_HOST)")
	}
	if c.IMAP.Username == "" {
		return fmt.Errorf("IMAP username is required (set GHOSTMAIL_IMAP_USERNAME)")
	}
	if c.IMAP.Password == "" {
		return fmt.Errorf("IMAP password is required (set GHOSTMAIL_IMAP_PASSWORD)")
	}
	if !c.IMAP.UseTLS && !c.IMAP.StartTLS && !c.IMAP.AllowInsecure {
		return fmt.Errorf("IMAP requires TLS or STARTTLS (set GHOSTMAIL_IMAP_USE_TLS=true or GHOSTMAIL_IMAP_STARTTLS=true; GHOSTMAIL_IMAP_ALLOW_INSECURE=true permits plaintext credentials and is unsafe)")
	}
	return nil
}

// getEnv retrieves an environment variable or returns a default value.
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// getEnvAsInt retrieves an environment variable as an integer.
func getEnvAsInt(key string, defaultValue int) int {
	valueStr := getEnv(key, "")
	if valueStr == "" {
		return defaultValue
	}
	value, err := strconv.Atoi(valueStr)
	if err != nil {
		return defaultValue
	}
	return value
}

// getEnvAsBool retrieves an environment variable as a boolean.
func getEnvAsBool(key string, defaultValue bool) bool {
	valueStr := getEnv(key, "")
	if valueStr == "" {
		return defaultValue
	}
	value, err := strconv.ParseBool(valueStr)
	if err != nil {
		return defaultValue
	}
	return value
}
