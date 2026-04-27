package security

import (
	"crypto/rand"
	"fmt"
	"os"
	"path/filepath"
)

// SecureWipe overwrites a file with random data before deleting it.
// Implements GDPR Art. 17 (Right to Erasure) with secure deletion.
// The file is overwritten once with random data, then truncated, then removed.
// This prevents recovery from filesystem journals or undelete tools.
func SecureWipe(path string) error {
	return secureWipeImpl(path, os.Stat, os.OpenFile, os.Remove)
}

// secureWipeImpl is the injectable implementation for testing.
func secureWipeImpl(
	path string,
	statFn func(string) (os.FileInfo, error),
	openFn func(string, int, os.FileMode) (*os.File, error),
	removeFn func(string) error,
) error {
	info, err := statFn(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // Already gone
		}
		return fmt.Errorf("secure wipe: stat failed: %w", err)
	}

	if info.IsDir() {
		return fmt.Errorf("secure wipe: %s is a directory (use SecureWipeDir)", path)
	}

	size := info.Size()

	// Overwrite with random data
	if size > 0 {
		f, err := openFn(path, os.O_WRONLY, 0)
		if err != nil {
			return fmt.Errorf("secure wipe: open failed: %w", err)
		}

		randomData := make([]byte, size)
		if _, err := rand.Read(randomData); err != nil {
			f.Close()
			return fmt.Errorf("secure wipe: random generation failed: %w", err)
		}

		if _, err := f.Write(randomData); err != nil {
			f.Close()
			return fmt.Errorf("secure wipe: overwrite failed: %w", err)
		}

		// Sync to ensure data is flushed to disk
		f.Sync()

		// Truncate to zero
		f.Truncate(0)
		f.Sync()

		f.Close()
	}

	// Remove the file
	if err := removeFn(path); err != nil {
		return fmt.Errorf("secure wipe: remove failed: %w", err)
	}

	return nil
}

// SecureWipeDir recursively securely wipes all files in a directory, then removes it.
func SecureWipeDir(dirPath string) error {
	return secureWipeDirImpl(dirPath, os.Stat, os.OpenFile, os.Remove, os.RemoveAll)
}

// secureWipeDirImpl is the injectable implementation for testing.
func secureWipeDirImpl(
	dirPath string,
	statFn func(string) (os.FileInfo, error),
	openFn func(string, int, os.FileMode) (*os.File, error),
	removeFn func(string) error,
	removeAllFn func(string) error,
) error {
	info, err := statFn(dirPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("secure wipe dir: stat failed: %w", err)
	}

	if !info.IsDir() {
		return secureWipeImpl(dirPath, statFn, openFn, removeFn)
	}

	// Walk and wipe all files first
	var wipeErrors []error
	err = filepath.Walk(dirPath, func(path string, fi os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return nil // Skip errors, continue walking
		}
		if fi.IsDir() {
			return nil // Skip dirs, handle files only
		}

		if wipeErr := secureWipeImpl(path, statFn, openFn, removeFn); wipeErr != nil {
			wipeErrors = append(wipeErrors, wipeErr)
		}
		return nil
	})

	if err != nil {
		return fmt.Errorf("secure wipe dir: walk failed: %w", err)
	}

	// Remove the directory tree (now empty or with only dirs)
	if removeAllErr := removeAllFn(dirPath); removeAllErr != nil {
		return fmt.Errorf("secure wipe dir: remove failed: %w", removeAllErr)
	}

	if len(wipeErrors) > 0 {
		return fmt.Errorf("secure wipe dir: %d file(s) had errors", len(wipeErrors))
	}

	return nil
}
