package config

import (
	"os"

	"github.com/joho/godotenv"
)

type SyncMode string

const (
	ModeSimple  SyncMode = "simple"
	ModeComplex SyncMode = "complex"
)

type Config struct {
	SyncMode       SyncMode
	SourceURL      string
	DBDSN          string
	TableTemp      string
	TableProvinces string
	TableRegencies string
	TableDistricts string
	TableVillages  string
	CronSchedule   string
	LastCommitFile string
}

func LoadConfig(mode SyncMode) (*Config, error) {
	_ = godotenv.Load()

	var defaultURL string
	var defaultCommitFile string
	if mode == ModeComplex {
		defaultURL = "https://raw.githubusercontent.com/cahyadsn/wilayah/refs/heads/master/db/wilayah_level_1_2.sql"
		defaultCommitFile = ".last_sync_commit_complex"
	} else {
		defaultURL = "https://raw.githubusercontent.com/cahyadsn/wilayah/master/db/wilayah.sql"
		defaultCommitFile = ".last_sync_commit_simple"
	}

	return &Config{
		SyncMode:       mode,
		SourceURL:      getEnv("SOURCE_URL", defaultURL),
		DBDSN:          getEnv("DB_DSN", "postgres://postgres:postgres@localhost:5432/wilayah?sslmode=disable"),
		TableTemp:      getEnv("TABLE_TEMP_WILAYAH", "temp_wilayah"),
		TableProvinces: getEnv("TABLE_PROVINCES", "provinces"),
		TableRegencies: getEnv("TABLE_REGENCIES", "regencies"),
		TableDistricts: getEnv("TABLE_DISTRICTS", "districts"),
		TableVillages:  getEnv("TABLE_VILLAGES", "villages"),
		CronSchedule:   getEnv("CRON_SCHEDULE", "0 0 * * *"),
		LastCommitFile: getEnv("LAST_COMMIT_FILE", defaultCommitFile),
	}, nil
}

func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}
