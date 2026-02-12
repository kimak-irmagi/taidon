package prepare

import "testing"

func TestLiquibaseFingerprintStable(t *testing.T) {
	sets := []LiquibaseChangeset{
		{ID: "1", Author: "alice", Path: "db/1.sql", SQLHash: "aaa"},
		{ID: "2", Author: "bob", Path: "db/2.sql", SQLHash: "bbb"},
	}
	a := liquibaseFingerprint("state-1", sets)
	b := liquibaseFingerprint("state-1", sets)
	if a == "" || a != b {
		t.Fatalf("expected stable fingerprint, got %q and %q", a, b)
	}
}

func TestLiquibaseFingerprintChangesOnOrder(t *testing.T) {
	a := liquibaseFingerprint("state-1", []LiquibaseChangeset{
		{ID: "1", Author: "alice", Path: "db/1.sql", SQLHash: "aaa"},
		{ID: "2", Author: "bob", Path: "db/2.sql", SQLHash: "bbb"},
	})
	b := liquibaseFingerprint("state-1", []LiquibaseChangeset{
		{ID: "2", Author: "bob", Path: "db/2.sql", SQLHash: "bbb"},
		{ID: "1", Author: "alice", Path: "db/1.sql", SQLHash: "aaa"},
	})
	if a == b {
		t.Fatalf("expected different fingerprints for different order")
	}
}

func TestLiquibaseFingerprintChangesOnPrevState(t *testing.T) {
	sets := []LiquibaseChangeset{{ID: "1", Author: "alice", Path: "db/1.sql", SQLHash: "aaa"}}
	a := liquibaseFingerprint("state-1", sets)
	b := liquibaseFingerprint("state-2", sets)
	if a == b {
		t.Fatalf("expected different fingerprints for different prev_state_id")
	}
}

func TestLiquibaseFingerprintChangesOnSQLHash(t *testing.T) {
	a := liquibaseFingerprint("state-1", []LiquibaseChangeset{
		{ID: "1", Author: "alice", Path: "db/1.sql", SQLHash: "aaa"},
	})
	b := liquibaseFingerprint("state-1", []LiquibaseChangeset{
		{ID: "1", Author: "alice", Path: "db/1.sql", SQLHash: "ccc"},
	})
	if a == b {
		t.Fatalf("expected different fingerprints for different sql_hash")
	}
}
