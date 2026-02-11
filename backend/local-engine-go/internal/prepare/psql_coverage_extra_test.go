package prepare

import "testing"

func TestIsConnectionFlagShortWithValue(t *testing.T) {
	cases := []string{"-hlocalhost", "-p5432", "-Uuser", "-ddb"}
	for _, arg := range cases {
		if !isConnectionFlag(arg) {
			t.Fatalf("expected connection flag for %q", arg)
		}
	}
}

func TestPreparePsqlArgsVarFlagError(t *testing.T) {
	if _, err := preparePsqlArgs([]string{"-vON_ERROR_STOP=0"}, nil); err == nil {
		t.Fatalf("expected error for ON_ERROR_STOP=0")
	}
}

func TestPreparePsqlArgsFileFlagLongEqError(t *testing.T) {
	if _, err := preparePsqlArgs([]string{"--file=rel.sql"}, nil); err == nil {
		t.Fatalf("expected error for relative file path")
	}
}
