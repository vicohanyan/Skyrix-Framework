package logger

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNewLoggerIndependentConfigurationAndCleanup(t *testing.T) {
	dir := t.TempDir()
	first, closeFirst := NewLogger(Config{LogOutput: "file", LogDir: dir, LogFile: "first.json", LogType: "json", LogLevel: "debug"})
	t.Cleanup(closeFirst)
	second, closeSecond := NewLogger(Config{LogOutput: "file", LogDir: dir, LogFile: "second.txt", LogType: "text", LogLevel: "error"})
	t.Cleanup(closeSecond)
	if first == second {
		t.Fatal("constructors returned the same logger")
	}
	first.With("component", "http").Debug("first message", "number", 7)
	second.Info("filtered")
	second.With("component", "worker").Error("second message", "error", errors.New("failure"))
	read := func(name string) []byte {
		t.Helper()
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Fatal(err)
		}
		return data
	}
	data := read("first.json")
	lines := bytes.Split(bytes.TrimSpace(data), []byte("\n"))
	if len(lines) != 2 {
		t.Fatalf("first output = %s", data)
	}
	var record map[string]any
	if err := json.Unmarshal(lines[1], &record); err != nil {
		t.Fatal(err)
	}
	if record["level"] != "DEBUG" || record["component"] != "http" || record["number"] != float64(7) {
		t.Fatalf("record = %#v", record)
	}
	text := string(read("second.txt"))
	if !strings.Contains(text, "level=ERROR") || !strings.Contains(text, "component=worker") || !strings.Contains(text, "error=failure") || strings.Contains(text, "filtered") || strings.Contains(text, "first message") {
		t.Fatalf("second output = %s", text)
	}
	closeFirst()
	first.Error("after cleanup")
	if !bytes.Equal(data, read("first.json")) {
		t.Fatal("first file remains writable after cleanup")
	}
	second.Error("still open")
	if !strings.Contains(string(read("second.txt")), "still open") {
		t.Fatal("closing first logger affected second")
	}
	closeSecond()
	before := read("second.txt")
	second.Error("after cleanup")
	if !bytes.Equal(before, read("second.txt")) {
		t.Fatal("second file remains writable after cleanup")
	}
}

func TestLevelsAndFormats(t *testing.T) {
	for _, level := range []struct {
		name string
		want slog.Level
	}{
		{"debug", slog.LevelDebug}, {"info", slog.LevelInfo}, {"", slog.LevelInfo}, {" WARN ", slog.LevelWarn}, {"warning", slog.LevelWarn}, {"error", slog.LevelError}, {"unknown", slog.LevelInfo},
	} {
		t.Run(level.name, func(t *testing.T) {
			for _, format := range []string{"json", "text", " TEXT ", "", "unknown"} {
				var out bytes.Buffer
				log := createLogger(format, &out, parseLogLevel(level.name))
				for _, candidate := range []slog.Level{slog.LevelDebug, slog.LevelInfo, slog.LevelWarn, slog.LevelError} {
					if log.Enabled(context.Background(), candidate) != (candidate >= level.want) {
						t.Fatalf("%q threshold for %s", format, candidate)
					}
					log.Log(context.Background(), candidate, "message", "value", 42)
				}
				lines := bytes.Split(bytes.TrimSpace(out.Bytes()), []byte("\n"))
				wantCount := int((slog.LevelError-level.want)/4) + 1
				if len(lines) != wantCount {
					t.Fatalf("%q output = %s", format, out.Bytes())
				}
				for _, line := range lines {
					if strings.EqualFold(strings.TrimSpace(format), "text") {
						if !bytes.Contains(line, []byte("value=42")) {
							t.Fatalf("text = %s", line)
						}
					} else {
						var record map[string]any
						if err := json.Unmarshal(line, &record); err != nil {
							t.Fatal(err)
						}
						if record["value"] != float64(42) {
							t.Fatalf("JSON = %s", line)
						}
					}
				}
			}
		})
	}
}

func TestOutputSelectionAndFallback(t *testing.T) {
	dir := t.TempDir()
	blocker := filepath.Join(dir, "blocker")
	if err := os.WriteFile(blocker, []byte("file"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name, output, dir, file string
		want                    *os.File
		fail                    bool
	}{
		{name: "default", want: os.Stdout},
		{name: "stdout", output: " STDOUT ", want: os.Stdout},
		{name: "alias", output: "std", want: os.Stdout},
		{name: "stderr", output: "stderr", want: os.Stderr},
		{name: "unknown", output: "other", want: os.Stdout, fail: true},
		{name: "missing filename", output: "file", want: os.Stdout, fail: true},
		{name: "mkdir failure", output: "file", dir: blocker, file: "sub/test.log", want: os.Stdout, fail: true},
		{name: "open failure", output: "file", file: dir, want: os.Stdout, fail: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			out, cleanup, err := getLogOutput(test.output, test.dir, test.file)
			if (err != nil) != test.fail || out != test.want {
				t.Fatalf("output=%v, error=%v", out, err)
			}
			cleanup()
			if _, err := test.want.Stat(); err != nil {
				t.Fatalf("cleanup closed standard stream: %v", err)
			}
		})
	}
	for _, absolute := range []bool{false, true} {
		name := "nested/test.log"
		if absolute {
			name = filepath.Join(dir, "absolute.log")
		}
		out, cleanup, err := getLogOutput("file", dir, name)
		if err != nil {
			t.Fatal(err)
		}
		file, ok := out.(*os.File)
		if !ok {
			t.Fatal("file output is not a file")
		}
		if _, err := file.WriteString("existing\n"); err != nil {
			t.Fatal(err)
		}
		cleanup()
		if _, err := file.WriteString("closed"); !errors.Is(err, os.ErrClosed) {
			t.Fatalf("write after cleanup = %v", err)
		}
		out, cleanup, err = getLogOutput("file", dir, name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := out.Write([]byte("appended\n")); err != nil {
			t.Fatal(err)
		}
		cleanup()
		data, err := os.ReadFile(file.Name())
		if err != nil {
			t.Fatal(err)
		}
		if string(data) != "existing\nappended\n" {
			t.Fatalf("file contents=%q", data)
		}
	}
}
