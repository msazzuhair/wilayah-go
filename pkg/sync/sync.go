package sync

import (
	"bufio"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"strings"
	"wilayah-go/pkg/config"
)

type GitHubCommit struct {
	Sha string `json:"sha"`
}

func GetLatestCommitSHA(sourceURL string) (string, error) {
	// Extract path from URL: https://raw.githubusercontent.com/user/repo/branch/path
	parts := strings.Split(sourceURL, "/")
	// Expected parts: [https: "" "raw.githubusercontent.com" "user" "repo" "branch" "path..."]
	if len(parts) < 7 {
		return "", fmt.Errorf("invalid source URL format")
	}
	user := parts[3]
	repo := parts[4]
	path := strings.Join(parts[6:], "/")

	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/commits?path=%s&page=1&per_page=1", user, repo, path)
	resp, err := http.Get(url)
	if err != nil {
		return "", err
	}
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {

		}
	}(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("github api returned status: %s", resp.Status)
	}

	var commits []GitHubCommit
	if err := json.NewDecoder(resp.Body).Decode(&commits); err != nil {
		return "", err
	}

	if len(commits) == 0 {
		return "", fmt.Errorf("no commits found for the specified path")
	}

	return commits[0].Sha, nil
}

func ReadStoredSHA(filePath string) string {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

func WriteStoredSHA(filePath, sha string) error {
	return os.WriteFile(filePath, []byte(sha), 0644)
}

func SynchronizeData(db *sql.DB, cfg *config.Config) error {
	latestSHA, err := GetLatestCommitSHA(cfg.SourceURL)
	if err != nil {
		return fmt.Errorf("failed to check for updates: %w", err)
	}

	storedSHA := ReadStoredSHA(cfg.LastCommitFile)
	if latestSHA == storedSHA {
		fmt.Println("Already up to date.")
		return nil
	}

	fmt.Printf("New version detected (%s). Starting sync for mode %s...\n", latestSHA, cfg.SyncMode)

	resp, err := http.Get(cfg.SourceURL)
	if err != nil {
		return fmt.Errorf("failed to download SQL: %w", err)
	}
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {

		}
	}(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to download SQL, status: %s", resp.Status)
	}

	// Prepare temp table
	if _, err := db.Exec(fmt.Sprintf("DROP TABLE IF EXISTS %s", cfg.TableTemp)); err != nil {
		return err
	}

	var createQuery string
	if cfg.SyncMode == config.ModeComplex {
		createQuery = fmt.Sprintf(`CREATE TABLE %s (
			code VARCHAR(13) PRIMARY KEY,
			name VARCHAR(100),
			capital VARCHAR(100),
			lat DOUBLE PRECISION,
			lng DOUBLE PRECISION,
			elv FLOAT,
			tz INT,
			luas DOUBLE PRECISION,
			penduduk DOUBLE PRECISION,
			path TEXT,
			status INT
		)`, cfg.TableTemp)
	} else {
		createQuery = fmt.Sprintf("CREATE TABLE %s (code VARCHAR(13) PRIMARY KEY, name VARCHAR(100))", cfg.TableTemp)
	}

	if _, err := db.Exec(createQuery); err != nil {
		return err
	}

	// Parse and execute SQL
	if err := processSQL(db, resp.Body, cfg.TableTemp, cfg.SyncMode); err != nil {
		return fmt.Errorf("failed to process SQL: %w", err)
	}

	// Sync to dedicated tables
	if err := syncFromTemp(db, cfg); err != nil {
		return fmt.Errorf("failed to sync from temp table: %w", err)
	}

	// Cleanup
	if _, err := db.Exec(fmt.Sprintf("DROP TABLE IF EXISTS %s", cfg.TableTemp)); err != nil {
		fmt.Printf("Warning: failed to drop temp table: %v\n", err)
	}

	if err := WriteStoredSHA(cfg.LastCommitFile, latestSHA); err != nil {
		return fmt.Errorf("failed to update stored SHA: %w", err)
	}

	fmt.Println("Sync completed successfully.")
	return nil
}

func processSQL(db *sql.DB, reader io.Reader, tempTableName string, mode config.SyncMode) error {
	scanner := bufio.NewScanner(reader)
	// Increase scanner buffer for large polygon data
	const maxCapacity = 10 * 1024 * 1024 // 10MB
	buf := make([]byte, 64*1024)
	scanner.Buffer(buf, maxCapacity)

	insertRegex := regexp.MustCompile(`(?i)INSERT INTO\s+.*?\s*\(.*?\)`)

	var currentValues []string
	inInsert := false
	totalRows := 0

	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer func(tx *sql.Tx) {
		_ = tx.Rollback()
	}(tx)

	cols := "(code, name)"
	if mode == config.ModeComplex {
		cols = "(code, name, capital, lat, lng, elv, tz, luas, penduduk, path, status)"
	}

	for scanner.Scan() {
		line := scanner.Text()
		trimmedLine := strings.TrimSpace(line)

		if insertRegex.MatchString(trimmedLine) {
			inInsert = true
			continue
		}

		if inInsert {
			if trimmedLine == "" || strings.HasPrefix(trimmedLine, "--") || strings.HasPrefix(trimmedLine, "/*") || strings.EqualFold(trimmedLine, "VALUES") {
				continue
			}

			cleaned := strings.TrimRight(trimmedLine, ";,")
			if cleaned != "" && strings.HasPrefix(cleaned, "(") {
				cleaned = strings.ReplaceAll(cleaned, `\'`, `''`)
				currentValues = append(currentValues, cleaned)
				totalRows++
			}

			if strings.HasSuffix(trimmedLine, ";") {
				if len(currentValues) > 0 {
					query := fmt.Sprintf("INSERT INTO %s %s VALUES %s", tempTableName, cols, strings.Join(currentValues, ","))
					if _, err := tx.Exec(query); err != nil {
						return fmt.Errorf("failed to execute bulk insert: %w", err)
					}
					currentValues = nil
				}
				inInsert = false
			} else if len(currentValues) >= 200 {
				query := fmt.Sprintf("INSERT INTO %s %s VALUES %s", tempTableName, cols, strings.Join(currentValues, ","))
				if _, err := tx.Exec(query); err != nil {
					return err
				}
				currentValues = nil
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return err
	}

	if len(currentValues) > 0 {
		query := fmt.Sprintf("INSERT INTO %s %s VALUES %s", tempTableName, cols, strings.Join(currentValues, ","))
		if _, err := tx.Exec(query); err != nil {
			return err
		}
	}

	fmt.Printf("Processed %d rows from SQL source.\n", totalRows)
	return tx.Commit()
}

func syncFromTemp(db *sql.DB, cfg *config.Config) error {
	var provinceSet, regencySet string
	fkSuffix := cfg.PKName

	if cfg.SyncMode == config.ModeComplex {
		extraSet := `
			capital = EXCLUDED.capital,
			lat = EXCLUDED.lat,
			lng = EXCLUDED.lng,
			elevation = EXCLUDED.elevation,
			timezone = EXCLUDED.timezone,
			area = EXCLUDED.area,
			population = EXCLUDED.population,
			boundary = EXCLUDED.boundary,
			status = EXCLUDED.status
		`
		provinceCols := fmt.Sprintf("(%s, name, capital, lat, lng, elevation, timezone, area, population, boundary, status)", cfg.PKName)
		provinceSelect := "code, name, capital, lat, lng, elv, tz, luas, penduduk, path, status"
		provinceSet = "name = EXCLUDED.name, " + extraSet

		regencyCols := fmt.Sprintf("(%s, name, province_%s, capital, lat, lng, elevation, timezone, area, population, boundary, status)", cfg.PKName, fkSuffix)
		regencySelect := "code, name, SUBSTRING(code, 1, 2), capital, lat, lng, elv, tz, luas, penduduk, path, status"
		regencySet = fmt.Sprintf("name = EXCLUDED.name, province_%s = EXCLUDED.province_%s, %s", fkSuffix, fkSuffix, extraSet)

		fmt.Println("Syncing provinces (complex)...")
		_, err := db.Exec(fmt.Sprintf(`
			INSERT INTO %s %s
			SELECT %s FROM %s WHERE LENGTH(code) = 2
			ON CONFLICT (%s) DO UPDATE SET %s
		`, cfg.TableProvinces, provinceCols, provinceSelect, cfg.TableTemp, cfg.PKName, provinceSet))
		if err != nil {
			return err
		}

		fmt.Println("Syncing regencies (complex)...")
		_, err = db.Exec(fmt.Sprintf(`
			INSERT INTO %s %s
			SELECT %s FROM %s WHERE LENGTH(code) = 5
			ON CONFLICT (%s) DO UPDATE SET %s
		`, cfg.TableRegencies, regencyCols, regencySelect, cfg.TableTemp, cfg.PKName, regencySet))
		if err != nil {
			return err
		}
	} else {
		// Simple Mode
		fmt.Println("Syncing provinces...")
		_, err := db.Exec(fmt.Sprintf(`
			INSERT INTO %s (%s, name)
			SELECT code, name FROM %s WHERE LENGTH(code) = 2
			ON CONFLICT (%s) DO UPDATE SET name = EXCLUDED.name
		`, cfg.TableProvinces, cfg.PKName, cfg.TableTemp, cfg.PKName))
		if err != nil {
			return err
		}

		fmt.Println("Syncing regencies...")
		_, err = db.Exec(fmt.Sprintf(`
			INSERT INTO %s (%s, name, province_%s)
			SELECT code, name, SUBSTRING(code, 1, 2) FROM %s WHERE LENGTH(code) = 5
			ON CONFLICT (%s) DO UPDATE SET name = EXCLUDED.name, province_%s = EXCLUDED.province_%s
		`, cfg.TableRegencies, cfg.PKName, fkSuffix, cfg.TableTemp, cfg.PKName, fkSuffix, fkSuffix))
		if err != nil {
			return err
		}

		fmt.Println("Syncing districts...")
		_, err = db.Exec(fmt.Sprintf(`
			INSERT INTO %s (%s, name, regency_%s)
			SELECT code, name, SUBSTRING(code, 1, 5) FROM %s WHERE LENGTH(code) = 8
			ON CONFLICT (%s) DO UPDATE SET name = EXCLUDED.name, regency_%s = EXCLUDED.regency_%s
		`, cfg.TableDistricts, cfg.PKName, fkSuffix, cfg.TableTemp, cfg.PKName, fkSuffix, fkSuffix))
		if err != nil {
			return err
		}

		fmt.Println("Syncing villages...")
		_, err = db.Exec(fmt.Sprintf(`
			INSERT INTO %s (%s, name, district_%s)
			SELECT code, name, SUBSTRING(code, 1, 8) FROM %s WHERE LENGTH(code) = 13
			ON CONFLICT (%s) DO UPDATE SET name = EXCLUDED.name, district_%s = EXCLUDED.district_%s
		`, cfg.TableVillages, cfg.PKName, fkSuffix, cfg.TableTemp, cfg.PKName, fkSuffix, fkSuffix))
		if err != nil {
			return err
		}
	}

	return nil
}
