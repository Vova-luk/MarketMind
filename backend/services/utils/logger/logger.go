package logger

import (
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
)

type Config struct {
	Level  string `yaml:"level"`
	Output string `yaml:"output"`
	Format string `yaml:"format"`
	File   string `yaml:"file"`
}

func (c Config) Validate() error {
	var level slog.Level
	if err := level.UnmarshalText([]byte(c.Level)); err != nil {
		return errors.New("logger.level must be debug, info, warn or error")
	}
	if c.Output != "stdout" && c.Output != "file" {
		return errors.New("logger.output must be stdout or file")
	}
	if c.Format != "text" && c.Format != "json" {
		return errors.New("logger.format must be text or json")
	}
	if c.Output == "file" && c.File == "" {
		return errors.New("logger.file is required for file output")
	}
	return nil
}

func New(cfg Config) (*slog.Logger, func() error, error) {
	if err := cfg.Validate(); err != nil {
		return nil, nil, err
	}
	var writer io.Writer = os.Stdout
	closeOutput := func() error { return nil }
	if cfg.Output == "file" {
		if err := os.MkdirAll(filepath.Dir(cfg.File), 0o750); err != nil {
			return nil, nil, fmt.Errorf("create log directory: %w", err)
		}
		file, err := os.OpenFile(cfg.File, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o640)
		if err != nil {
			return nil, nil, fmt.Errorf("open log file: %w", err)
		}
		writer, closeOutput = file, file.Close
	}
	var level slog.Level
	_ = level.UnmarshalText([]byte(cfg.Level))
	opts := &slog.HandlerOptions{Level: level}
	var handler slog.Handler = slog.NewTextHandler(writer, opts)
	if cfg.Format == "json" {
		handler = slog.NewJSONHandler(writer, opts)
	}
	return slog.New(handler), closeOutput, nil
}
