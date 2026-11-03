// Package atlastest provides a fake Atlas CLI for tests of db migrate:diff.
// It implements only "migrate diff <name> --config file://<config> --env
// <env>" and writes the SQL file without atlas.sum: the framework computes
// the checksum itself with the Atlas Go library. It does not prove the
// integration with the real CLI; tests that need it check for atlas in PATH.
//
// Environment variables control the fake:
//
//	FAKE_ATLAS_LOG      file that receives one line per invocation
//	FAKE_ATLAS_FAIL     "diff" makes diff fail after writing a partial file
//	FAKE_ATLAS_SQL      SQL that diff writes; empty means no schema changes
//	FAKE_ATLAS_VERSION  version of the generated file (default 20261103000000)
//	FAKE_ATLAS_CONFIG   file that receives a copy of the diff config
package atlastest

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const script = `#!/bin/sh
set -eu
export LC_ALL=C
if [ -n "${FAKE_ATLAS_LOG:-}" ]; then echo "$*" >> "$FAKE_ATLAS_LOG"; fi
case "$1 $2" in
"migrate diff")
	if [ -n "${FAKE_ATLAS_CONFIG:-}" ]; then cp "${5#file://}" "$FAKE_ATLAS_CONFIG"; fi
	dir=$(sed -n 's/^ *dir = "file:\/\/\(.*\)"$/\1/p' "${5#file://}")
	if [ "${FAKE_ATLAS_FAIL:-}" = diff ]; then
		echo partial > "$dir/${FAKE_ATLAS_VERSION:-20261103000000}_$3.sql"
		echo "fake atlas: diff failed" >&2
		exit 1
	fi
	if [ -z "${FAKE_ATLAS_SQL:-}" ]; then echo "no changes"; exit 0; fi
	printf '%s\n' "$FAKE_ATLAS_SQL" > "$dir/${FAKE_ATLAS_VERSION:-20261103000000}_$3.sql"
	;;
*)
	echo "fake atlas: unsupported $*" >&2
	exit 2
	;;
esac
`

// Install writes the fake atlas binary into a temporary directory, puts that
// directory first in PATH and returns the binary path and the invocation log.
func Install(t testing.TB) (binary, log string) {
	t.Helper()
	dir := t.TempDir()
	binary = filepath.Join(dir, "atlas")
	if err := os.WriteFile(binary, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	log = filepath.Join(dir, "invocations.log")
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("FAKE_ATLAS_LOG", log)
	t.Setenv("FAKE_ATLAS_FAIL", "")
	t.Setenv("FAKE_ATLAS_SQL", "")
	t.Setenv("FAKE_ATLAS_VERSION", "")
	t.Setenv("FAKE_ATLAS_CONFIG", "")
	return binary, log
}

// Invocations returns the logged invocations of the fake.
func Invocations(t testing.TB, log string) []string {
	t.Helper()
	data, err := os.ReadFile(log)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		t.Fatal(err)
	}
	return strings.Split(strings.TrimSpace(string(data)), "\n")
}

// Snapshot returns the regular files under root with their contents, keyed by
// slash-separated relative path, and the directories (with a trailing slash).
func Snapshot(t testing.TB, root string) map[string]string {
	t.Helper()
	files := map[string]string{}
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(root, path)
		rel = filepath.ToSlash(rel)
		if entry.IsDir() {
			files[rel+"/"] = ""
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		files[rel] = string(data)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return files
}
