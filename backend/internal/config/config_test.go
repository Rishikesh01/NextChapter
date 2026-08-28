package config

import "testing"

func TestCookieSecureTriState(t *testing.T) {
	t.Run("unset leaves the override nil", func(t *testing.T) {
		// Explicitly neutralize any ambient value from the dev shell;
		// FromEnv treats empty as unset.
		t.Setenv("NEXTCHAPTER_COOKIE_SECURE", "")
		cfg, err := FromEnv()
		if err != nil {
			t.Fatalf("FromEnv: %v", err)
		}
		if cfg.CookieSecure != nil {
			t.Fatalf("CookieSecure = %v, want nil", *cfg.CookieSecure)
		}
	})

	for _, tc := range []struct {
		value string
		want  bool
	}{
		{"true", true},
		{"1", true},
		{"false", false},
		{"0", false},
	} {
		t.Run("explicit "+tc.value, func(t *testing.T) {
			t.Setenv("NEXTCHAPTER_COOKIE_SECURE", tc.value)
			cfg, err := FromEnv()
			if err != nil {
				t.Fatalf("FromEnv: %v", err)
			}
			if cfg.CookieSecure == nil || *cfg.CookieSecure != tc.want {
				t.Fatalf("CookieSecure = %v, want %v", cfg.CookieSecure, tc.want)
			}
		})
	}

	t.Run("garbage is a config error", func(t *testing.T) {
		t.Setenv("NEXTCHAPTER_COOKIE_SECURE", "yes-please")
		if _, err := FromEnv(); err == nil {
			t.Fatal("FromEnv accepted an unparsable NEXTCHAPTER_COOKIE_SECURE")
		}
	})
}
