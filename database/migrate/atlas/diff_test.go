package atlas

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

type diffCapture struct {
	Args                    []string
	ConfigPath, Config, CWD string
	Mode                    os.FileMode
}

func TestMain(m *testing.M) {
	if os.Getenv("SKYRIX_ATLAS_DIFF_HELPER") == "1" {
		if err := captureDiff(); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(8)
		}
		if os.Getenv("SKYRIX_ATLAS_DIFF_FAIL") == "1" {
			os.Exit(7)
		}
		if os.Getenv("SKYRIX_ATLAS_DIFF_BLOCK") == "1" {
			time.Sleep(time.Hour)
		}
		os.Exit(0)
	}
	os.Exit(m.Run())
}
func captureDiff() error {
	args := os.Args[1:]
	if len(args) != 7 || args[0] != "migrate" || args[1] != "diff" || args[3] != "--config" || args[5] != "--env" {
		return fmt.Errorf("unexpected args: %v", args)
	}
	u, err := url.Parse(args[4])
	if err != nil {
		return err
	}
	if u.Scheme != "file" || !filepath.IsAbs(u.Path) {
		return fmt.Errorf("invalid config URL")
	}
	data, err := os.ReadFile(u.Path)
	if err != nil {
		return err
	}
	info, err := os.Stat(u.Path)
	if err != nil {
		return err
	}
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	encoded, err := json.Marshal(diffCapture{Args: args, ConfigPath: u.Path, Config: string(data), CWD: cwd, Mode: info.Mode().Perm()})
	if err != nil {
		return err
	}
	return os.WriteFile("capture.json", encoded, 0600)
}

func TestDiffUsesTemporaryConfigAndCleansUp(t *testing.T) {
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	for _, mode := range []string{"success", "failure", "cancel"} {
		t.Run(mode, func(t *testing.T) {
			root := t.TempDir()
			temporary := filepath.Join(t.TempDir(), "private temporary configs")
			if err := os.Mkdir(temporary, 0700); err != nil {
				t.Fatal(err)
			}
			t.Setenv("TMPDIR", temporary)
			t.Setenv("SKYRIX_ATLAS_DIFF_HELPER", "1")
			t.Setenv("SKYRIX_ATLAS_DIFF_FAIL", "0")
			t.Setenv("SKYRIX_ATLAS_DIFF_BLOCK", "0")
			ctx := context.Background()
			if mode == "failure" {
				t.Setenv("SKYRIX_ATLAS_DIFF_FAIL", "1")
			}
			if mode == "cancel" {
				t.Setenv("SKYRIX_ATLAS_DIFF_BLOCK", "1")
				var cancel context.CancelFunc
				ctx, cancel = context.WithTimeout(ctx, 500*time.Millisecond)
				defer cancel()
			}
			// A pre-existing user Atlas config is ignored and preserved.
			legacy := filepath.Join(root, "atlas.hcl")
			if err := os.WriteFile(legacy, []byte("user-owned config"), 0600); err != nil {
				t.Fatal(err)
			}
			err := Diff(ctx, executable, root, "add_field", DiffOptions{Env: "production", DevURL: "postgres://user:secret@localhost/scratch?search_path=Scratch"}, io.Discard, io.Discard)
			if (err == nil) != (mode == "success") {
				t.Fatalf("mode=%s error=%v", mode, err)
			}
			var got diffCapture
			data, readErr := os.ReadFile(filepath.Join(root, "capture.json"))
			if readErr != nil {
				t.Fatal(readErr)
			}
			if err := json.Unmarshal(data, &got); err != nil {
				t.Fatal(err)
			}
			if got.CWD != root || got.Mode != 0600 {
				t.Fatalf("cwd=%q mode=%o", got.CWD, got.Mode)
			}
			if _, err := os.Stat(got.ConfigPath); !os.IsNotExist(err) {
				t.Fatalf("temporary config remains: %v", err)
			}
			if entries, err := os.ReadDir(temporary); err != nil || len(entries) != 0 {
				t.Fatal("temporary files remain")
			}
			for _, want := range []string{`working_dir = ` + hclString(root), `env "production"`, `dev = "postgres://user:secret@localhost/scratch?search_path=Scratch"`, hclString(fileURL(filepath.Join(root, "migrations"))), `program = ["go", "run", "./tools/schemaloader"]`} {
				if !strings.Contains(got.Config, want) {
					t.Fatalf("missing %q in config:\n%s", want, got.Config)
				}
			}
			if !reflect.DeepEqual(got.Args, []string{"migrate", "diff", "add_field", "--config", fileURL(got.ConfigPath), "--env", "production"}) {
				t.Fatal(got.Args)
			}
			unchanged, _ := os.ReadFile(legacy)
			if string(unchanged) != "user-owned config" {
				t.Fatal("legacy config modified")
			}
		})
	}
}

func TestDiffDefaultsNeedNoProjectConfig(t *testing.T) {
	t.Setenv("SKYRIX_ATLAS_DIFF_HELPER", "1")
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	if err := Diff(context.Background(), executable, root, "initial", DiffOptions{}, io.Discard, io.Discard); err != nil {
		t.Fatal(err)
	}
	var got diffCapture
	data, err := os.ReadFile(filepath.Join(root, "capture.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got.Config, `env "local"`) || !strings.Contains(got.Config, hclString(DefaultDevURL)) {
		t.Fatal(got.Config)
	}
	if _, err := os.Stat(filepath.Join(root, "atlas.hcl")); !os.IsNotExist(err) {
		t.Fatal("created project config")
	}
}
func TestHCLLiteralEscapes(t *testing.T) {
	for _, test := range []struct{ input, want string }{
		{"Mixed Case", `"Mixed Case"`},
		{"\"\\\n\r\t\x00", `"\"\\\n\r\t\u0000"`},
		{"${env(\"SECRET\")} %{if true}", `"$${env(\"SECRET\")} %%{if true}"`},
	} {
		if got := hclString(test.input); got != test.want {
			t.Fatalf("quote(%q)=%s want %s", test.input, got, test.want)
		}
	}
}
