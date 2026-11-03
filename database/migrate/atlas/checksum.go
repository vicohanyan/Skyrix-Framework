package atlas

import (
	"fmt"

	atlasmigrate "ariga.io/atlas/sql/migrate"
)

// Sum returns the atlas.sum contents for the SQL migrations in dir. It is
// computed by the official Atlas Go library (ariga.io/atlas/sql/migrate), the
// same code "atlas migrate hash" uses, so the file stays compatible with the
// Atlas CLI. No Atlas CLI, account or database is needed.
func Sum(dir string) ([]byte, error) {
	local, err := atlasmigrate.NewLocalDir(dir)
	if err != nil {
		return nil, fmt.Errorf("open migration directory: %w", err)
	}
	hash, err := local.Checksum()
	if err != nil {
		return nil, fmt.Errorf("compute %s: %w", SumFile, err)
	}
	data, err := hash.MarshalText()
	if err != nil {
		return nil, fmt.Errorf("encode %s: %w", SumFile, err)
	}
	return data, nil
}
