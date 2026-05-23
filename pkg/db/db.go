package db

import (
	"database/sql"
	"fmt"
	"wilayah-go/pkg/config"

	_ "github.com/lib/pq"
)

func InitDB(cfg *config.Config) (*sql.DB, error) {
	db, err := sql.Open("postgres", cfg.DBDSN)
	if err != nil {
		return nil, err
	}

	if err := db.Ping(); err != nil {
		return nil, err
	}

	if err := ensureSchema(db, cfg); err != nil {
		return nil, err
	}

	return db, nil
}

func ensureSchema(db *sql.DB, cfg *config.Config) error {
	var provinceCols, regencyCols string

	if cfg.SyncMode == config.ModeComplex {
		extraCols := `
			capital VARCHAR(100),
			lat DOUBLE PRECISION,
			lng DOUBLE PRECISION,
			elevation FLOAT,
			timezone INT,
			area DOUBLE PRECISION,
			population DOUBLE PRECISION,
			boundary TEXT,
			status INT
		`
		provinceCols = "code VARCHAR(2) PRIMARY KEY, name VARCHAR(100) NOT NULL, " + extraCols
		regencyCols = "code VARCHAR(5) PRIMARY KEY, name VARCHAR(100) NOT NULL, province_code VARCHAR(2) REFERENCES " + cfg.TableProvinces + "(code), " + extraCols
	} else {
		provinceCols = "code VARCHAR(2) PRIMARY KEY, name VARCHAR(100) NOT NULL"
		regencyCols = "code VARCHAR(5) PRIMARY KEY, name VARCHAR(100) NOT NULL, province_code VARCHAR(2) REFERENCES " + cfg.TableProvinces + "(code)"
	}

	queries := []string{
		fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (%s)`, cfg.TableProvinces, provinceCols),
		fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (%s)`, cfg.TableRegencies, regencyCols),
	}

	// Districts and Villages are only for simple mode (as per current source)
	// But let's keep them anyway as they don't hurt.
	queries = append(queries,
		fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (
			code VARCHAR(8) PRIMARY KEY,
			name VARCHAR(100) NOT NULL,
			regency_code VARCHAR(5) REFERENCES %s(code)
		)`, cfg.TableDistricts, cfg.TableRegencies),
		fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (
			code VARCHAR(13) PRIMARY KEY,
			name VARCHAR(100) NOT NULL,
			district_code VARCHAR(8) REFERENCES %s(code)
		)`, cfg.TableVillages, cfg.TableDistricts),
	)

	for _, q := range queries {
		if _, err := db.Exec(q); err != nil {
			return fmt.Errorf("failed to execute query %s: %w", q, err)
		}
	}

	return nil
}
