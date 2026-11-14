// Package config reads the server configuration from the environment.
package config

import (
	"errors"
	"fmt"
	"net/url"

	"github.com/caarlos0/env/v11"
)

// Version is the app version, set at build time with
// -ldflags "-X github.com/wolf29f/hushtongue/internal/config.Version=…".
var Version = "dev"

// Listen is the part of the configuration the healthcheck needs, read alone
// so that it doesn't require the other variables.
type Listen struct {
	Addr string `env:"HUSHTONGUE_ADDR" envDefault:":8080"`
}

// Config is the server configuration.
type Config struct {
	Listen

	DatabaseURL     string   `env:"DATABASE_URL,required,notEmpty"`
	GMUsername      string   `env:"HUSHTONGUE_GM_USERNAME" envDefault:"mj"`
	GMPassword      string   `env:"HUSHTONGUE_GM_PASSWORD,required,notEmpty"`
	PublicURL       *url.URL `env:"HUSHTONGUE_PUBLIC_URL"` // nil when deduced from each request
	SourceLangLabel string   `env:"HUSHTONGUE_SOURCE_LANG_LABEL" envDefault:"Français"`
	ConLangLabel    string   `env:"HUSHTONGUE_CON_LANG_LABEL" envDefault:"conLang"`
}

// Load reads the configuration from environment, or from the process
// environment when nil. The error lists every missing or invalid variable.
func Load(environment map[string]string) (Config, error) {
	// The parse goes on past an error, so the URL check still applies
	cfg, err := env.ParseAsWithOptions[Config](env.Options{Environment: environment})
	if cfg.PublicURL != nil &&
		((cfg.PublicURL.Scheme != "http" && cfg.PublicURL.Scheme != "https") ||
			cfg.PublicURL.Host == "") {
		err = errors.Join(err, fmt.Errorf("HUSHTONGUE_PUBLIC_URL %q must be an http:// or https:// URL", cfg.PublicURL))
	}
	if err != nil {
		return Config{}, err
	}
	return cfg, nil
}

// LoadListen reads the listening part of the configuration from environment,
// or from the process environment when nil.
func LoadListen(environment map[string]string) (Listen, error) {
	return env.ParseAsWithOptions[Listen](env.Options{Environment: environment})
}
