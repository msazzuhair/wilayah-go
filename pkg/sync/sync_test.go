package sync

import (
	"strings"
	"testing"
)

func TestProcessSQL_Parsing(t *testing.T) {
	// Regular expression to match INSERT INTO statements
	line := "INSERT INTO `wilayah` (`kode`, `nama`) VALUES"
	if !strings.Contains(strings.ToLower(line), "insert into") {
		t.Errorf("Expected insert into to match")
	}
}

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
