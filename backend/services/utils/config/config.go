// Package config provides configuration helpers shared by backend services.
package config

import (
	"fmt"
	"strings"
	"time"

	"github.com/joho/godotenv"
	"github.com/knadh/koanf/parsers/yaml"
	"github.com/knadh/koanf/providers/env/v2"
	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/v2"
)

// Reader exposes configuration values without exposing the underlying library.
type Reader struct {
	source *koanf.Koanf
}

func (r *Reader) String(path string) string { return r.source.String(path) }

func (r *Reader) Int(path string) int { return r.source.Int(path) }

func (r *Reader) Duration(path string) time.Duration { return r.source.Duration(path) }

func (r *Reader) Unmarshal(path string, target any) error {
	return r.source.UnmarshalWithConf(path, target, koanf.UnmarshalConf{Tag: "yaml"})
}

func Load(path, envPrefix string) (*Reader, error) {
	reader := koanf.New(".")
	if err := reader.Load(file.Provider(path), yaml.Parser()); err != nil {
		return nil, fmt.Errorf("load config: %w", err)
	}
	if envPrefix != "" {
		if err := reader.Load(env.Provider(".", env.Opt{
			Prefix: envPrefix,
			TransformFunc: func(key, value string) (string, any) {
				key = strings.ToLower(strings.TrimPrefix(key, envPrefix))
				return strings.ReplaceAll(key, "__", "."), value
			},
		}), nil); err != nil {
			return nil, fmt.Errorf("load prefixed config environment: %w", err)
		}
	}
	return &Reader{source: reader}, nil
}

func LoadEnv(path string) error {
	if path == "" {
		return nil
	}
	if err := godotenv.Load(path); err != nil {
		return fmt.Errorf("load env file: %w", err)
	}
	return nil
}
