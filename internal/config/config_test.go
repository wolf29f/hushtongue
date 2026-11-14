package config_test

import (
	"strings"
	"testing"

	"github.com/wolf29f/hushtongue/internal/config"
)

func TestLoad(t *testing.T) {
	t.Run("applies the defaults", func(t *testing.T) {
		cfg, err := config.Load(map[string]string{
			"DATABASE_URL":           "postgres://localhost/hushtongue",
			"HUSHTONGUE_GM_PASSWORD": "secret",
		})
		if err != nil {
			t.Fatalf("Load() failed: %v", err)
		}

		if cfg.GMUsername != "mj" || cfg.Addr != ":8080" || cfg.PublicURL != nil ||
			cfg.SourceLangLabel != "Français" || cfg.ConLangLabel != "conLang" {
			t.Errorf("Load() = %+v, want the defaults", cfg)
		}
	})

	t.Run("reads every variable", func(t *testing.T) {
		cfg, err := config.Load(map[string]string{
			"DATABASE_URL":                 "postgres://localhost/hushtongue",
			"HUSHTONGUE_GM_PASSWORD":       "secret",
			"HUSHTONGUE_GM_USERNAME":       "wolf",
			"HUSHTONGUE_PUBLIC_URL":        "https://hushtongue.example.com",
			"HUSHTONGUE_ADDR":              "127.0.0.1:9000",
			"HUSHTONGUE_SOURCE_LANG_LABEL": "English",
			"HUSHTONGUE_CON_LANG_LABEL":    "Islander",
		})
		if err != nil {
			t.Fatalf("Load() failed: %v", err)
		}

		if cfg.DatabaseURL != "postgres://localhost/hushtongue" || cfg.GMPassword != "secret" ||
			cfg.GMUsername != "wolf" || cfg.PublicURL.String() != "https://hushtongue.example.com" ||
			cfg.Addr != "127.0.0.1:9000" || cfg.SourceLangLabel != "English" || cfg.ConLangLabel != "Islander" {
			t.Errorf("Load() = %+v, want every variable", cfg)
		}
	})

	t.Run("lists every missing or invalid variable", func(t *testing.T) {
		_, err := config.Load(map[string]string{
			"HUSHTONGUE_PUBLIC_URL": "hushtongue.example.com",
		})
		if err == nil {
			t.Fatal("Load() succeeded, want an error")
		}

		for _, variable := range []string{"DATABASE_URL", "HUSHTONGUE_GM_PASSWORD", "HUSHTONGUE_PUBLIC_URL"} {
			if !strings.Contains(err.Error(), variable) {
				t.Errorf("Load() error = %q, want it to name %s", err, variable)
			}
		}
	})
}

func TestLoadListen(t *testing.T) {
	listen, err := config.LoadListen(map[string]string{"HUSHTONGUE_ADDR": "127.0.0.1:9000"})
	if err != nil || listen.Addr != "127.0.0.1:9000" {
		t.Errorf("LoadListen() = %+v, %v, want 127.0.0.1:9000 without the other variables", listen, err)
	}
}
