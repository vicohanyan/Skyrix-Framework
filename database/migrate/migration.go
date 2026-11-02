// Package migrate applies forward-only SQL migrations.
//
// A migration is a single SQL file named in the Atlas directory format:
// <YYYYMMDDHHMMSS>_<name>.sql. Files ending in .notx.sql run outside a
// transaction (for statements such as CREATE INDEX CONCURRENTLY). There are no
// down migrations: a mistake is corrected by a new migration.
//
// The package depends only on the standard library and database/sql.
package migrate

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/fs"
	"path"
	"regexp"
	"sort"
	"strings"
	"time"
)

const (
	// VersionLayout is the time layout of a migration version.
	VersionLayout = "20060102150405"

	// AtlasSumFile is the Atlas directory checksum file; the runner ignores it.
	AtlasSumFile = "atlas.sum"

	noTxSuffix = ".notx.sql"
	sqlSuffix  = ".sql"
)

var (
	fileNamePattern = regexp.MustCompile(`^([0-9]{14})_([a-z0-9][a-z0-9_]*)(\.notx)?\.sql$`)
	namePattern     = regexp.MustCompile(`^[a-z0-9][a-z0-9_]*$`)
)

// Migration is a single SQL migration file.
type Migration struct {
	Version  string
	Name     string
	NoTx     bool
	FileName string
	SQL      string
	Checksum string
}

// ID returns the migration identifier used in messages: <version>_<name>.
func (m Migration) ID() string { return m.Version + "_" + m.Name }

// FileName returns the file name for a migration version and name.
func FileName(version, name string, noTx bool) (string, error) {
	if err := ValidateVersion(version); err != nil {
		return "", err
	}
	if err := ValidateName(name); err != nil {
		return "", err
	}
	if noTx {
		return version + "_" + name + noTxSuffix, nil
	}
	return version + "_" + name + sqlSuffix, nil
}

// NewVersion returns the migration version for the given time in UTC.
func NewVersion(now time.Time) string {
	return now.UTC().Format(VersionLayout)
}

// ValidateVersion checks that version is a valid <YYYYMMDDHHMMSS> timestamp.
func ValidateVersion(version string) error {
	if len(version) != len(VersionLayout) {
		return fmt.Errorf("migration version %q must have the format YYYYMMDDHHMMSS", version)
	}
	if _, err := time.Parse(VersionLayout, version); err != nil {
		return fmt.Errorf("migration version %q must have the format YYYYMMDDHHMMSS", version)
	}
	return nil
}

// ValidateName checks that name contains only lowercase ASCII letters, digits
// and underscores.
func ValidateName(name string) error {
	if !namePattern.MatchString(name) {
		return fmt.Errorf("migration name %q must contain only lowercase letters, digits and underscores", name)
	}
	return nil
}

// ParseFileName parses a migration file name.
func ParseFileName(fileName string) (version, name string, noTx bool, err error) {
	match := fileNamePattern.FindStringSubmatch(fileName)
	if match == nil {
		return "", "", false, fmt.Errorf("migration file %q must be named <YYYYMMDDHHMMSS>_<name>.sql or <YYYYMMDDHHMMSS>_<name>.notx.sql", fileName)
	}
	if err := ValidateVersion(match[1]); err != nil {
		return "", "", false, fmt.Errorf("migration file %q: %w", fileName, err)
	}
	return match[1], match[2], match[3] != "", nil
}

// Load reads migrations from the root of source, sorted by version.
//
// Directories and atlas.sum are ignored. Any other file must be a valid
// migration file; duplicate versions and empty files are errors.
func Load(source fs.FS) ([]Migration, error) {
	if source == nil {
		return nil, fmt.Errorf("migration source is nil")
	}
	entries, err := fs.ReadDir(source, ".")
	if err != nil {
		return nil, fmt.Errorf("read migration source: %w", err)
	}

	byVersion := make(map[string]string, len(entries))
	migrations := make([]Migration, 0, len(entries))
	for _, entry := range entries {
		fileName := entry.Name()
		if entry.IsDir() || fileName == AtlasSumFile {
			continue
		}
		version, name, noTx, err := ParseFileName(fileName)
		if err != nil {
			return nil, err
		}
		if existing, ok := byVersion[version]; ok {
			return nil, fmt.Errorf("duplicate migration version %s: %s and %s", version, existing, fileName)
		}
		byVersion[version] = fileName

		content, err := fs.ReadFile(source, path.Join(".", fileName))
		if err != nil {
			return nil, fmt.Errorf("read migration %s: %w", fileName, err)
		}
		if strings.TrimSpace(string(content)) == "" {
			return nil, fmt.Errorf("migration %s is empty", fileName)
		}
		sum := sha256.Sum256(content)
		migrations = append(migrations, Migration{
			Version:  version,
			Name:     name,
			NoTx:     noTx,
			FileName: fileName,
			SQL:      string(content),
			Checksum: hex.EncodeToString(sum[:]),
		})
	}

	sort.Slice(migrations, func(i, j int) bool {
		return migrations[i].Version < migrations[j].Version
	})
	return migrations, nil
}
