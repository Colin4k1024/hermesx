package evolution

// Config controls the Oris evolution integration.
type Config struct {
	Enabled          bool                           `yaml:"enabled"`
	StorageMode      string                         `yaml:"storage_mode"` // "sqlite" (default) | "mysql"
	DBPath           string                         `yaml:"db_path"`      // default ~/.hermes/evolution.db
	MySQLDSN         string                         `yaml:"mysql_dsn"`
	MinConfidence    float64                        `yaml:"min_confidence"`   // default 0.5 — minimum to inject
	ReplayThreshold  float64                        `yaml:"replay_threshold"` // default 0.75 — skip LLM if above
	MaxGenesInPrompt int                            `yaml:"max_genes_prompt"` // default 3
	SharingMode      string                         `yaml:"sharing_mode"`     // "disabled" (default) | "anonymous" | "trusted"
	TenantPolicies   map[string]TenantSharingPolicy `yaml:"tenant_policies"`
}

// DefaultConfig returns sensible defaults for the evolution system.
func DefaultConfig() Config {
	return Config{
		Enabled:          false,
		StorageMode:      "sqlite",
		MinConfidence:    0.5,
		ReplayThreshold:  0.75,
		MaxGenesInPrompt: 3,
		SharingMode:      SharingDisabled,
	}
}
