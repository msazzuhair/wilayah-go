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
		_ = db.Close()
		return nil, err
	}

	if err := ensureSchema(db, cfg); err != nil {
		_ = db.Close()
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
		provinceCols = fmt.Sprintf("%s VARCHAR(2) PRIMARY KEY, name VARCHAR(100) NOT NULL, %s", cfg.PKName, extraCols)
		regencyCols = fmt.Sprintf("%s VARCHAR(5) PRIMARY KEY, name VARCHAR(100) NOT NULL, province_code VARCHAR(2) REFERENCES %s(%s), %s", cfg.PKName, cfg.TableProvinces, cfg.PKName, extraCols)
	} else {
		provinceCols = fmt.Sprintf("%s VARCHAR(2) PRIMARY KEY, name VARCHAR(100) NOT NULL", cfg.PKName)
		regencyCols = fmt.Sprintf("%s VARCHAR(5) PRIMARY KEY, name VARCHAR(100) NOT NULL, province_code VARCHAR(2) REFERENCES %s(%s)", cfg.PKName, cfg.TableProvinces, cfg.PKName)
	}

	queries := []string{
		fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (%s)`, cfg.TableProvinces, provinceCols),
		fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (%s)`, cfg.TableRegencies, regencyCols),
	}

	// Districts and Villages are only for simple mode (as per current source)
	// But let's keep them anyway as they don't hurt.
	queries = append(queries,
		fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (
			%s VARCHAR(8) PRIMARY KEY,
			name VARCHAR(100) NOT NULL,
			regency_code VARCHAR(5) REFERENCES %s(%s)
		)`, cfg.TableDistricts, cfg.PKName, cfg.TableRegencies, cfg.PKName),
		fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (
			%s VARCHAR(13) PRIMARY KEY,
			name VARCHAR(100) NOT NULL,
			district_code VARCHAR(8) REFERENCES %s(%s)
		)`, cfg.TableVillages, cfg.PKName, cfg.TableDistricts, cfg.PKName),
	)

	for _, q := range queries {
		if _, err := db.Exec(q); err != nil {
			return fmt.Errorf("failed to execute query %s: %w", q, err)
		}
	}

	return nil
}
