package config

import (
	"log"

	validate "scheduler/internal/validate"
	util "scheduler/pkg/util"

	"github.com/alecthomas/kingpin/v2"
	_ "github.com/jsternberg/zap-logfmt"
	"github.com/knadh/koanf"
	"github.com/knadh/koanf/parsers/yaml"
	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/providers/rawbytes"
)

// Load reads the config file, unmarshals it into Config, and validates it.
// Fatal on any parse, unmarshal, or validation error.
func Load() Config {
	configPath := kingpin.Flag("config", "Path To The Application Config File").
		Short('c').Default("").String()
	kingpin.Parse()

	k := koanf.New(".")
	if err := k.Load(rawbytes.Provider(DefaultConfig), yaml.Parser()); err != nil {
		log.Fatalf("Failed to load default config: %v", err)
	}
	if *configPath != "" {
		if err := k.Load(file.Provider(*configPath), yaml.Parser()); err != nil {
			log.Fatalf("Failed to load config file %s: %v", *configPath, err)
		}
	}

	var cfg Config
	if err := k.Unmarshal("", &cfg); err != nil {
		log.Fatalf("Error unmarshalling config: %v", err)
	}

	if !cfg.IsProdMode {
		util.PrintStruct(cfg)
	}

	if err := cfg.Validate(); err != nil {
		validate.LogValidationErrors(err)
		log.Fatalf("Invalid Configuration!")
	}

	return cfg
}
