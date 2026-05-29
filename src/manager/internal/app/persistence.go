package app

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"

	"github.com/MikeO7/threadgate/src/manager/internal/config"
)

var varLibThreadPath = "/var/lib/thread"

// ensureStatePersistence redirects /var/lib/thread to the persistent /data/otbr volume.
// If /var/lib/thread is not already a symlink, any existing credentials are migrated to
// /data/otbr first before replacing the directory with a symlink.
func ensureStatePersistence(cfg *config.Config) error {
	if cfg.Runtime.IsMock() {
		log.Println("[Persistence] Mock mode: skipping state directory redirect.")
		return nil
	}

	varLibThread := varLibThreadPath
	persistentThreadDir := filepath.Join(cfg.StateDir, "otbr")

	info, err := os.Lstat(varLibThread)
	if err != nil {
		if os.IsNotExist(err) {
			log.Printf("[Persistence] %s does not exist. Preparing automatic redirection...\n", varLibThread)
			return createSymlinkOnly(persistentThreadDir, varLibThread)
		}
		return fmt.Errorf("failed to stat %s: %w", varLibThread, err)
	}

	if info.Mode()&os.ModeSymlink != 0 {
		log.Printf("[Persistence] %s is already a symlink to persistent storage. State is secure.\n", varLibThread)
		return nil
	}

	return migrateNativeDirectory(varLibThread, persistentThreadDir)
}

func migrateNativeDirectory(varLibThread, persistentThreadDir string) error {
	log.Printf("[Persistence] Found native directory at %s. Migrating state to %s...\n", varLibThread, persistentThreadDir)

	if err := os.MkdirAll(persistentThreadDir, 0o750); err != nil {
		return fmt.Errorf("failed to create persistent directory %s: %w", persistentThreadDir, err)
	}

	files, err := os.ReadDir(varLibThread)
	if err != nil {
		return fmt.Errorf("failed to read native directory %s: %w", varLibThread, err)
	}

	for _, file := range files {
		if file.IsDir() {
			continue
		}
		srcPath := filepath.Join(varLibThread, file.Name())
		dstPath := filepath.Join(persistentThreadDir, file.Name())
		log.Printf("[Persistence] Migrating file: %s -> %s\n", file.Name(), dstPath)
		if err := copyFile(srcPath, dstPath); err != nil {
			return fmt.Errorf("failed to migrate file %s: %w", file.Name(), err)
		}
	}

	if err := os.RemoveAll(varLibThread); err != nil {
		return fmt.Errorf("failed to remove native directory %s: %w", varLibThread, err)
	}

	if err := os.Symlink(persistentThreadDir, varLibThread); err != nil {
		return fmt.Errorf("failed to create symlink from %s to %s: %w", varLibThread, persistentThreadDir, err)
	}

	log.Printf("[Persistence] Successfully migrated and symlinked %s -> %s\n", varLibThread, persistentThreadDir)
	return nil
}

func createSymlinkOnly(target, link string) error {
	if err := os.MkdirAll(target, 0o750); err != nil {
		return fmt.Errorf("failed to create target directory %s: %w", target, err)
	}
	if err := os.MkdirAll(filepath.Dir(link), 0o750); err != nil {
		return fmt.Errorf("failed to create link parent directory %s: %w", filepath.Dir(link), err)
	}
	if err := os.Symlink(target, link); err != nil {
		return fmt.Errorf("failed to create symlink: %w", err)
	}
	log.Printf("[Persistence] Redirected %s -> %s\n", link, target)
	return nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src) //nolint:gosec // G304: src is a file entry read from /var/lib/thread during migration
	if err != nil {
		return err
	}
	defer func() { _ = in.Close() }()

	out, err := os.Create(dst) //nolint:gosec // G304: dst is the persistent otbr directory under configured StateDir
	if err != nil {
		return err
	}
	defer func() { _ = out.Close() }()

	_, err = io.Copy(out, in)
	return err
}
