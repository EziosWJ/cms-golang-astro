package migrations_test

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"golang.org/x/crypto/bcrypt"
)

var (
	gooseUpMarker   = regexp.MustCompile(`(?m)^-- \+goose Up\s*$`)
	gooseDownMarker = regexp.MustCompile(`(?m)^-- \+goose Down\s*$`)
	// Schema migrations own structure only. Row mutation (UPDATE/DELETE/MERGE)
	// and seed-style INSERT ... VALUES stay forbidden.
	//
	// An exception is carved out for INSERT INTO <table> (...) SELECT ...:
	// SQLite cannot drop an inline column constraint, so structural table
	// rebuilds must copy existing rows across a CREATE TABLE swap. Without that
	// escape hatch the rebuild is structurally impossible. Seed data still
	// belongs to the seed stream, so INSERT ... VALUES remains rejected here.
	schemaMutation = regexp.MustCompile(`(?im)^\s*(UPDATE|DELETE|MERGE)\b`)
	schemaSeedData = regexp.MustCompile(`(?is)\bINSERT\b[^;]*?\bVALUES\b`)
	seedDDL        = regexp.MustCompile(`(?im)^\s*(CREATE|ALTER|DROP|TRUNCATE)\b`)
	bcryptHash     = regexp.MustCompile(`\$2[aby]\$[0-9]{2}\$[./A-Za-z0-9]{53}`)
)

func TestMigrationStreamsHaveValidGooseFiles(t *testing.T) {
	for _, dialect := range []string{"postgres", "sqlite"} {
		schemaFiles := migrationFiles(t, dialectPath(dialect, "schema"))
		if len(schemaFiles) == 0 {
			t.Fatalf("%s schema migration stream must contain a baseline", dialect)
		}
		assertGooseFiles(t, schemaFiles)
		assertGooseFiles(t, migrationFiles(t, dialectPath(dialect, "seed")))
	}
}

func TestAdminSeedPasswordMatchesJavaContract(t *testing.T) {
	for _, dialect := range []string{"postgres", "sqlite"} {
		contents := readFile(t, filepath.Join(dialectPath(dialect, "seed"), "00001_auth_seed.sql"))
		hash := bcryptHash.Find(contents)
		if hash == nil {
			t.Fatalf("%s authentication seed must contain a BCrypt password hash", dialect)
		}
		if err := bcrypt.CompareHashAndPassword(hash, []byte("admin123")); err != nil {
			t.Fatalf("%s administrator seed password does not match admin123: %v", dialect, err)
		}
	}
}

func TestSchemaStreamStillRejectsSeedStyleInserts(t *testing.T) {
	seedStyle := []byte("INSERT INTO cms_site_config(id,version) VALUES(1,1);")
	if !schemaSeedData.Match(seedStyle) {
		t.Error("INSERT ... VALUES must stay forbidden in the schema stream")
	}

	rebuild := []byte("INSERT INTO cms_site_config (id, version)\n  SELECT id, version FROM cms_site_config_legacy;")
	if schemaSeedData.Match(rebuild) {
		t.Error("the INSERT ... SELECT rebuild exception must be permitted")
	}
	if schemaMutation.Match(rebuild) {
		t.Error("a structural rebuild is not a row mutation")
	}
	if !schemaMutation.Match([]byte("UPDATE cms_site_config SET status=1;")) {
		t.Error("UPDATE must stay forbidden in the schema stream")
	}
	if !schemaMutation.Match([]byte("DELETE FROM cms_site_config;")) {
		t.Error("DELETE must stay forbidden in the schema stream")
	}
}

func TestSchemaAndSeedResponsibilitiesStaySeparate(t *testing.T) {
	for _, dialect := range []string{"postgres", "sqlite"} {
		for _, name := range migrationFiles(t, dialectPath(dialect, "schema")) {
			contents := readFile(t, name)
			if schemaMutation.Match(contents) {
				t.Errorf("schema migration %s mutates rows", name)
			}
			if schemaSeedData.Match(contents) {
				t.Errorf("schema migration %s contains seed-data INSERT ... VALUES", name)
			}
		}

		for _, name := range migrationFiles(t, dialectPath(dialect, "seed")) {
			contents := readFile(t, name)
			if seedDDL.Match(contents) {
				t.Errorf("seed migration %s contains schema DDL", name)
			}
		}
	}
}

func TestDialectMigrationVersionsStayInLockstep(t *testing.T) {
	for _, kind := range []string{"schema", "seed"} {
		postgres := migrationVersions(t, dialectPath("postgres", kind))
		sqlite := migrationVersions(t, dialectPath("sqlite", kind))
		if len(postgres) != len(sqlite) {
			t.Fatalf("%s migration versions differ: postgres=%v sqlite=%v", kind, postgres, sqlite)
		}
		for version := range postgres {
			if !sqlite[version] {
				t.Errorf("SQLite %s stream is missing logical migration version %05d", kind, version)
			}
		}
		for version := range sqlite {
			if !postgres[version] {
				t.Errorf("PostgreSQL %s stream is missing logical migration version %05d", kind, version)
			}
		}
	}
}

func assertGooseFiles(t *testing.T, files []string) {
	t.Helper()
	for _, name := range files {
		contents := readFile(t, name)
		up := gooseUpMarker.FindIndex(contents)
		down := gooseDownMarker.FindIndex(contents)
		if up == nil || down == nil {
			t.Errorf("migration %s must contain exact Goose Up and Down markers", name)
			continue
		}
		if up[0] >= down[0] {
			t.Errorf("migration %s places Goose Down before Goose Up", name)
		}
	}
}

func migrationFiles(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read migration directory %s: %v", dir, err)
	}

	var files []string
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sql") {
			continue
		}
		files = append(files, filepath.Join(dir, entry.Name()))
	}
	sort.Strings(files)
	return files
}

func migrationVersions(t *testing.T, dir string) map[int]bool {
	t.Helper()
	versions := make(map[int]bool)
	for _, name := range migrationFiles(t, dir) {
		base := filepath.Base(name)
		var version int
		if _, err := fmt.Sscanf(base, "%05d_", &version); err != nil {
			t.Fatalf("parse migration version %s: %v", name, err)
		}
		versions[version] = true
	}
	return versions
}

func dialectPath(dialect, kind string) string {
	if dialect == "postgres" {
		return kind
	}
	return filepath.Join(dialect, kind)
}

func readFile(t *testing.T, name string) []byte {
	t.Helper()
	contents, err := os.ReadFile(name)
	if err != nil {
		t.Fatalf("read migration %s: %v", name, err)
	}
	return contents
}
