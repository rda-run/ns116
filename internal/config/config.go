package config

import (
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

type ServerConfig struct {
	Port int    `yaml:"port"`
	Host string `yaml:"host"`
}

type AWSConfig struct {
	AccessKeyID     string `yaml:"access_key_id"`
	SecretAccessKey string `yaml:"secret_access_key"`
	Region          string `yaml:"region"`
}

type HostedZoneEntry struct {
	ID    string `yaml:"id"`
	Label string `yaml:"label"`
}

type DatabaseConfig struct {
	DSN  string `yaml:"dsn"`
	Path string `yaml:"path"` // Kept for backwards compatibility but we will check DSN
}

type LDAPConfig struct {
	Enabled      bool              `yaml:"enabled"`
	URL          string            `yaml:"url"`
	BindDN       string            `yaml:"bind_dn"`
	BindPassword string            `yaml:"bind_password"`
	BaseDN       string            `yaml:"base_dn"`
	UserFilter   string            `yaml:"user_filter"`
	UsernameAttr string            `yaml:"username_attr"`
	EmailAttr    string            `yaml:"email_attr"`
	StartTLS     bool              `yaml:"starttls"`
	SkipVerify   bool              `yaml:"skip_verify"`
	GroupFilter  string            `yaml:"group_filter"` // Optional filter to find groups. Defaults to (|(member=%s)(uniqueMember=%s))
	GroupMapping map[string]string `yaml:"group_mapping"`
}

type Config struct {
	Server      ServerConfig      `yaml:"server"`
	AWS         AWSConfig         `yaml:"aws"`
	HostedZones []HostedZoneEntry `yaml:"hosted_zones"`
	Database    DatabaseConfig    `yaml:"database"`
	LDAP        LDAPConfig        `yaml:"ldap"`
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	if cfg.Server.Port == 0 {
		cfg.Server.Port = 8080
	}
	if cfg.Server.Host == "" {
		cfg.Server.Host = "0.0.0.0"
	}
	if cfg.AWS.Region == "" {
		cfg.AWS.Region = "us-east-1"
	}
	// Database config
	if cfg.Database.DSN == "" {
		// Default to local dev postgres if nothing provided
		cfg.Database.DSN = "postgres://ns116:ns116pass@localhost:5432/ns116?sslmode=disable"
	}

	if cfg.LDAP.Enabled {
		if cfg.LDAP.URL == "" {
			return nil, fmt.Errorf("ldap.url is required when LDAP is enabled")
		}
		if cfg.LDAP.BindDN == "" || cfg.LDAP.BindPassword == "" {
			return nil, fmt.Errorf("ldap.bind_dn and ldap.bind_password are required")
		}
		if cfg.LDAP.BaseDN == "" {
			return nil, fmt.Errorf("ldap.base_dn is required")
		}
		if len(cfg.LDAP.GroupMapping) == 0 {
			return nil, fmt.Errorf("ldap.group_mapping must define at least one role")
		}
		if cfg.LDAP.UserFilter == "" {
			cfg.LDAP.UserFilter = "(sAMAccountName=%s)"
		}
		if cfg.LDAP.UsernameAttr == "" {
			cfg.LDAP.UsernameAttr = "sAMAccountName"
		}
		if strings.HasPrefix(cfg.LDAP.URL, "ldap://") && !cfg.LDAP.StartTLS {
			// In a real logger we'd use log.Warn, but here just fmt to stdout or ignore
			// The plan says "log a warning at startup". Since Load returns config,
			// maybe the caller should log it?
			// For now, let's just leave it or print to stderr.
			// However, this function just loads config. The logging responsibility
			// is better placed in the server startup if needed, or we can fmt.Println.
			fmt.Println("WARNING: LDAP is configured with ldap:// but StartTLS is disabled. Credentials will be sent in cleartext.")
		}
	}

	return &cfg, nil
}
