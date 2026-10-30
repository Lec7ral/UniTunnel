// Package appstate stores UniTunnel's own runtime state (network snapshots) in a
// single per-user directory, so the app can restore the machine exactly as it
// was even after a crash or a forced kill.
package appstate

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

const dirName = "UniTunnel"

// Dir returns %LOCALAPPDATA%\UniTunnel (falling back to the user profile) and
// creates it with owner-only permissions.
func Dir() (string, error) {
	if base := os.Getenv("LOCALAPPDATA"); base != "" {
		dir := filepath.Join(base, dirName)
		if err := os.MkdirAll(dir, 0o700); err == nil {
			return dir, nil
		}
	}
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return "", fmt.Errorf("no se pudo determinar el directorio de estado de UniTunnel: %w", err)
	}
	dir := filepath.Join(home, dirName)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	return dir, nil
}

// Path returns the absolute path of a state file inside Dir.
func Path(name string) (string, error) {
	dir, err := Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, name), nil
}

// Exists reports whether the named state file is present.
func Exists(name string) (bool, error) {
	p, err := Path(name)
	if err != nil {
		return false, err
	}
	if _, err := os.Stat(p); err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// WriteJSON serialises v as indented JSON and writes it atomically.
func WriteJSON(name string, v any) error {
	p, err := Path(name)
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return WriteFileAtomic(p, data, 0o600)
}

// ReadJSON loads a previously written state file into v.
func ReadJSON(name string, v any) error {
	p, err := Path(name)
	if err != nil {
		return err
	}
	data, err := os.ReadFile(p)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, v)
}

// Remove deletes a state file; a missing file is not an error.
func Remove(name string) error {
	p, err := Path(name)
	if err != nil {
		return err
	}
	if err := os.Remove(p); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// WriteFileAtomic writes data to a temporary file in the destination directory
// and renames it over path. A crash can therefore never leave a truncated file
// that would later be parsed as a corrupt snapshot.
func WriteFileAtomic(path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".unitunnel-*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer func() {
		_ = tmp.Close()
		_ = os.Remove(tmpName) // no-op once the rename succeeded
	}()

	if err := tmp.Chmod(perm); err != nil {
		// Windows only understands read-only; ignore.
		_ = err
	}
	if _, err := tmp.Write(data); err != nil {
		return err
	}
	if err := tmp.Sync(); err != nil {
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, path)
}
