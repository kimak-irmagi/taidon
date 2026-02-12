package prepare

import "testing"

func TestParseLiquibaseChecksum(t *testing.T) {
	output := `-- Changeset config/liquibase/changelog/001.xml::001::dev
CREATE TABLE test(id INT);
INSERT INTO public.databasechangelog (ID, AUTHOR, FILENAME, DATEEXECUTED, ORDEREXECUTED, MD5SUM) VALUES ('001','dev','config/liquibase/changelog/001.xml', NOW(), 1, '9:abcdef');`
	changesets, err := parseLiquibaseUpdateSQL(output)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(changesets) != 1 {
		t.Fatalf("expected one changeset, got %d", len(changesets))
	}
	if changesets[0].Checksum != "9:abcdef" {
		t.Fatalf("expected checksum, got %q", changesets[0].Checksum)
	}
	if liquibaseChangesetHash(changesets[0]) != "9:abcdef" {
		t.Fatalf("expected checksum to be preferred")
	}
}

func TestParseLiquibaseChecksumFallbacksToSQL(t *testing.T) {
	output := `-- Changeset config/liquibase/changelog/002.xml::002::dev
CREATE TABLE test2(id INT);`
	changesets, err := parseLiquibaseUpdateSQL(output)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(changesets) != 1 {
		t.Fatalf("expected one changeset, got %d", len(changesets))
	}
	if changesets[0].Checksum != "" {
		t.Fatalf("expected empty checksum, got %q", changesets[0].Checksum)
	}
	if liquibaseChangesetHash(changesets[0]) != changesets[0].SQLHash {
		t.Fatalf("expected SQLHash fallback")
	}
}
