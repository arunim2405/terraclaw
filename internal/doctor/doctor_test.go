package doctor_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/arunim2405/terraclaw/config"
	"github.com/arunim2405/terraclaw/internal/doctor"
)

type fakeClient struct {
	schemas []string
	err     error
}

func (f fakeClient) ListSchemas() ([]string, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.schemas, nil
}

func (f fakeClient) Close() error {
	return nil
}

func TestRun_AllChecksPass(t *testing.T) {
	outDir := t.TempDir()
	cfg := &config.Config{
		TerraformBin:  "terraform",
		OutputDir:     outDir,
		SteampipeHost: "localhost",
		SteampipePort: "9193",
		SteampipeDB:   "steampipe",
		SteampipeUser: "steampipe",
	}

	deps := doctor.Deps{
		LookPath: func(file string) (string, error) {
			return "/usr/local/bin/" + file, nil
		},
		ConnectSteampipe: func(_ string) (doctor.Client, error) {
			return fakeClient{schemas: []string{"aws"}}, nil
		},
		MkdirAll:   os.MkdirAll,
		CreateTemp: os.CreateTemp,
		Remove:     os.Remove,
	}

	report := doctor.Run(cfg, deps)
	if report.HasFailures() {
		t.Fatalf("expected no failures, got %+v", report.Checks)
	}
}

func TestRun_FailuresDetected(t *testing.T) {
	base := t.TempDir()
	filePath := filepath.Join(base, "not-a-dir")
	if err := os.WriteFile(filePath, []byte("x"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	cfg := &config.Config{
		OutputDir:    filePath,
		TerraformBin: "terraform",
	}

	deps := doctor.Deps{
		LookPath: func(string) (string, error) {
			return "", errors.New("not found")
		},
		ConnectSteampipe: func(_ string) (doctor.Client, error) {
			return nil, errors.New("connection refused")
		},
		MkdirAll:   os.MkdirAll,
		CreateTemp: os.CreateTemp,
		Remove:     os.Remove,
	}

	report := doctor.Run(cfg, deps)
	if !report.HasFailures() {
		t.Fatalf("expected failures, got %+v", report.Checks)
	}
}

func TestRun_NoSteampipeSchemasFails(t *testing.T) {
	cfg := &config.Config{
		TerraformBin: "terraform",
		OutputDir:    t.TempDir(),
	}

	deps := doctor.Deps{
		LookPath: func(file string) (string, error) {
			return "/usr/local/bin/" + file, nil
		},
		ConnectSteampipe: func(_ string) (doctor.Client, error) {
			return fakeClient{schemas: nil}, nil
		},
		MkdirAll:   os.MkdirAll,
		CreateTemp: os.CreateTemp,
		Remove:     os.Remove,
	}

	report := doctor.Run(cfg, deps)
	if !report.HasFailures() {
		t.Fatalf("expected schema check failure, got %+v", report.Checks)
	}
}
