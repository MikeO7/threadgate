package hassdev

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const (
	fixtureConfigPrefix = "ha-config/"
	fixtureCredsName    = "ha-credentials.env"
)

// BuildFixture archives ha-config and credentials for fast reuse.
func BuildFixture(cfg Config) error {
	_, _ = fmt.Fprintf(os.Stdout, "==> Building fixture %s\n", cfg.FixtureFile)
	if err := os.MkdirAll(filepath.Dir(cfg.FixtureFile), dirPerm); err != nil {
		return err
	}
	f, err := os.Create(cfg.FixtureFile)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()

	gz := gzip.NewWriter(f)
	defer func() { _ = gz.Close() }()
	tw := tar.NewWriter(gz)
	defer func() { _ = tw.Close() }()

	if err := addTarDir(tw, fixtureConfigPrefix, cfg.HAConfigDir); err != nil {
		return err
	}
	return addTarFile(tw, fixtureCredsName, cfg.CredsFile)
}

// ApplyFixture extracts a golden HA fixture (HA should be stopped).
func ApplyFixture(cfg Config) error {
	_, _ = fmt.Fprintf(os.Stdout, "==> Applying fixture %s\n", cfg.FixtureFile)
	if err := prepareFixtureDirs(cfg); err != nil {
		return err
	}
	if err := extractFixtureArchive(cfg); err != nil {
		return err
	}
	cleanupStaleHAState(cfg.HAConfigDir)
	return nil
}

func prepareFixtureDirs(cfg Config) error {
	if err := os.RemoveAll(cfg.HAConfigDir); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(cfg.HAConfigDir), dirPerm); err != nil {
		return err
	}
	return os.MkdirAll(filepath.Dir(cfg.CredsFile), dirPerm)
}

func extractFixtureArchive(cfg Config) error {
	f, err := os.Open(cfg.FixtureFile)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	defer func() { _ = gz.Close() }()
	tr := tar.NewReader(gz)

	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		if err := applyFixtureTarEntry(tr, hdr, cfg); err != nil {
			return err
		}
	}
}

func applyFixtureTarEntry(tr *tar.Reader, hdr *tar.Header, cfg Config) error {
	switch {
	case strings.HasPrefix(hdr.Name, fixtureConfigPrefix):
		rel := strings.TrimPrefix(hdr.Name, fixtureConfigPrefix)
		if rel == "" {
			return nil
		}
		target := filepath.Join(cfg.HAConfigDir, rel)
		return extractTarEntry(tr, hdr, target)
	case hdr.Name == fixtureCredsName:
		return extractTarEntry(tr, hdr, cfg.CredsFile)
	default:
		return nil
	}
}

// cleanupStaleHAState removes files that prevent Home Assistant from starting after restore.
func cleanupStaleHAState(configDir string) {
	entries := []string{
		filepath.Join(configDir, ".ha_run.lock"),
		filepath.Join(configDir, "home-assistant_v2.db-wal"),
		filepath.Join(configDir, "home-assistant_v2.db-shm"),
	}
	for _, p := range entries {
		_ = os.Remove(p)
	}
}

func addTarDir(tw *tar.Writer, prefix, dir string) error {
	return filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		return appendWalkPathToTar(tw, prefix, dir, path, info)
	})
}

func appendWalkPathToTar(tw *tar.Writer, prefix, dir, path string, info os.FileInfo) error {
	if fixtureSkip(path, info) {
		if info.IsDir() {
			return filepath.SkipDir
		}
		return nil
	}
	rel, err := filepath.Rel(dir, path)
	if err != nil {
		return err
	}
	if rel == "." {
		return nil
	}
	name := prefix + filepath.ToSlash(rel)
	if err := writeTarHeader(tw, name, info); err != nil {
		return err
	}
	if info.IsDir() {
		return nil
	}
	return copyFileIntoTar(tw, path)
}

func writeTarHeader(tw *tar.Writer, name string, info os.FileInfo) error {
	hdr, err := tar.FileInfoHeader(info, "")
	if err != nil {
		return err
	}
	hdr.Name = name
	return tw.WriteHeader(hdr)
}

func copyFileIntoTar(tw *tar.Writer, path string) error {
	f, err := os.Open(path) //nolint:gosec // G304: path from filepath.Walk within fixture source dir
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()
	_, err = io.Copy(tw, f)
	return err
}

func fixtureSkip(path string, info os.FileInfo) bool {
	base := filepath.Base(path)
	if info.IsDir() {
		return false
	}
	switch {
	case strings.HasPrefix(base, "home-assistant.log"),
		strings.HasSuffix(base, ".db-wal"),
		strings.HasSuffix(base, ".db-shm"),
		base == ".ha_run.lock":
		return true
	}
	return false
}

func addTarFile(tw *tar.Writer, name, path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	if err := writeTarHeader(tw, name, info); err != nil {
		return err
	}
	return copyFileIntoTar(tw, path)
}

func extractTarEntry(tr *tar.Reader, hdr *tar.Header, target string) error {
	switch hdr.Typeflag {
	case tar.TypeDir:
		return os.MkdirAll(target, dirPerm)
	case tar.TypeReg:
		return extractRegularTarFile(tr, hdr, target)
	default:
		return nil
	}
}

func extractRegularTarFile(tr *tar.Reader, hdr *tar.Header, target string) error {
	if err := os.MkdirAll(filepath.Dir(target), dirPerm); err != nil {
		return err
	}
	out, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.FileMode(hdr.Mode)) //nolint:gosec // G304: target derived from fixture prefix + relative path
	if err != nil {
		return err
	}
	defer func() { _ = out.Close() }()
	_, err = io.Copy(out, tr)
	return err
}
