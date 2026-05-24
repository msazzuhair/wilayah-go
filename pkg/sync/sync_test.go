package sync

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"wilayah-go/pkg/config"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestHierarchyLogic(t *testing.T) {
	tests := []struct {
		code string
		want int
	}{
		{"11", 2},
		{"11.01", 5},
		{"11.01.01", 8},
		{"11.01.01.2001", 13},
	}

	for _, tt := range tests {
		if len(tt.code) != tt.want {
			t.Errorf("code %s length = %d; want %d", tt.code, len(tt.code), tt.want)
		}
	}
}

func TestProcessSQL_Simple(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to open mock database: %v", err)
	}
	defer db.Close()

	sqlData := `
INSERT INTO wilayah (kode, nama) VALUES
('11','Aceh'),
('11.01','Kabupaten Aceh Selatan');
`
	reader := strings.NewReader(sqlData)

	mock.ExpectBegin()
	mock.ExpectExec("INSERT INTO temp_wilayah").WithArgs().WillReturnResult(sqlmock.NewResult(1, 2))
	mock.ExpectCommit()

	err = processSQL(db, reader, "temp_wilayah", config.ModeSimple)
	if err != nil {
		t.Errorf("processSQL failed: %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled expectations: %s", err)
	}
}

func TestProcessSQL_Complex(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to open mock database: %v", err)
	}
	defer db.Close()

	// Complex SQL format (subset of columns for testing)
	sqlData := `
INSERT INTO wilayah_level_1_2 (kode, nama, ibukota, lat, lng, elv, tz, luas, penduduk, path, status) VALUES
('11','Aceh','Banda Aceh', 5.57, 95.34, 11, 7, 56835, 5623479, '[[...]]', 1);
`
	reader := strings.NewReader(sqlData)

	mock.ExpectBegin()
	mock.ExpectExec("INSERT INTO temp_wilayah").WithArgs().WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	err = processSQL(db, reader, "temp_wilayah", config.ModeComplex)
	if err != nil {
		t.Errorf("processSQL failed: %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled expectations: %s", err)
	}
}

func TestSyncFromTemp_Simple(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to open mock database: %v", err)
	}
	defer db.Close()

	cfg := &config.Config{
		SyncMode:       config.ModeSimple,
		TableTemp:      "temp_wilayah",
		TableProvinces: "provinces",
		TableRegencies: "regencies",
		TableDistricts: "districts",
		TableVillages:  "villages",
		PKName:         "code",
	}

	mock.ExpectExec("INSERT INTO provinces \\(code, name\\)").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec("INSERT INTO regencies \\(code, name, province_code\\)").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec("INSERT INTO districts \\(code, name, regency_code\\)").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec("INSERT INTO villages \\(code, name, district_code\\)").WillReturnResult(sqlmock.NewResult(1, 1))

	err = syncFromTemp(db, cfg)
	if err != nil {
		t.Errorf("syncFromTemp failed: %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled expectations: %s", err)
	}
}

func TestForceSync(t *testing.T) {
	sqlData := `INSERT INTO wilayah (kode, nama) VALUES ('11','Aceh');`
	mockSHA := "test-sha-123"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/commits") {
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprintf(w, `[{"sha": "%s"}]`, mockSHA)
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(sqlData))
	}))
	defer server.Close()

	os.Setenv("GITHUB_API_BASE", server.URL)
	defer os.Unsetenv("GITHUB_API_BASE")

	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to open mock database: %v", err)
	}
	defer db.Close()

	// Create a temporary commit file
	tmpFile := ".test_last_sync_commit"
	defer os.Remove(tmpFile)

	// Mock URL must have enough parts for GetLatestCommitSHA
	// raw.githubusercontent.com/user/repo/branch/db/wilayah.sql
	fakeSourceURL := server.URL + "/user/repo/branch/db/wilayah.sql"

	cfg := &config.Config{
		SyncMode:       config.ModeSimple,
		SourceURL:      fakeSourceURL,
		TableTemp:      "temp_wilayah",
		TableProvinces: "provinces",
		TableRegencies: "regencies",
		TableDistricts: "districts",
		TableVillages:  "villages",
		PKName:         "code",
		ForceSync:      true,
		LastCommitFile: tmpFile,
	}

	// 1. Setup Table
	mock.ExpectExec("DROP TABLE IF EXISTS temp_wilayah").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec("CREATE TABLE temp_wilayah").WillReturnResult(sqlmock.NewResult(1, 1))

	// 2. Data Insertion into temp
	mock.ExpectBegin()
	mock.ExpectExec("INSERT INTO temp_wilayah").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	// 3. syncFromTemp execution
	mock.ExpectExec("INSERT INTO provinces").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec("INSERT INTO regencies").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec("INSERT INTO districts").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec("INSERT INTO villages").WillReturnResult(sqlmock.NewResult(1, 1))

	// 4. Cleanup
	mock.ExpectExec("DROP TABLE IF EXISTS temp_wilayah").WillReturnResult(sqlmock.NewResult(1, 1))

	err = SynchronizeData(db, cfg)
	if err != nil {
		t.Errorf("SynchronizeData with ForceSync=true failed: %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled expectations: %s", err)
	}

	// Verify the SHA was written
	storedSHA := ReadStoredSHA(tmpFile)
	if storedSHA != mockSHA {
		t.Errorf("expected stored SHA %s, got %s", mockSHA, storedSHA)
	}
}

func TestSyncFromTemp_Complex(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to open mock database: %v", err)
	}
	defer db.Close()

	cfg := &config.Config{
		SyncMode:       config.ModeComplex,
		TableTemp:      "temp_wilayah",
		TableProvinces: "provinces",
		TableRegencies: "regencies",
		PKName:         "id",
	}

	mock.ExpectExec("INSERT INTO provinces \\(id,").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec("INSERT INTO regencies \\(id, name, province_id,").WillReturnResult(sqlmock.NewResult(1, 1))

	err = syncFromTemp(db, cfg)
	if err != nil {
		t.Errorf("syncFromTemp failed: %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled expectations: %s", err)
	}
}
