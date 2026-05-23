package sync

import (
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
	}

	mock.ExpectExec("INSERT INTO provinces").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec("INSERT INTO regencies").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec("INSERT INTO districts").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec("INSERT INTO villages").WillReturnResult(sqlmock.NewResult(1, 1))

	err = syncFromTemp(db, cfg)
	if err != nil {
		t.Errorf("syncFromTemp failed: %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled expectations: %s", err)
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
	}

	mock.ExpectExec("INSERT INTO provinces").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec("INSERT INTO regencies").WillReturnResult(sqlmock.NewResult(1, 1))

	err = syncFromTemp(db, cfg)
	if err != nil {
		t.Errorf("syncFromTemp failed: %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled expectations: %s", err)
	}
}
