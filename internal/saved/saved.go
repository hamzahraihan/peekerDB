package saved

import (
	"encoding/json"
	"fmt"

	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Profile is one named connection: a DBeaver-style saved entry.
type Profile struct {
	Name      string    `json:"name"`
	Conn      string    `json:"conn"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Store persists profiles as JSON under the OS user config dir.
// Passwords live in the conn strings, so the file is created 0600.
type Store struct {
	Path     string
	profiles []Profile
	pathErr  error // why Path is empty, when the config dir was unavailable
	loadErr  error // why the file could not be read; such a store must not save
}

// encryptPayload encrypts the store before it is written. It is a
// variable so a test can force the failure that would otherwise only
// happen on a Windows host whose user key is unavailable.
var encryptPayload = encryptPayloadPlain

// decryptPayload reverses encryptPayload.
var decryptPayload = decryptPayloadPlain

// Warn reports something the user should know about their saved
// connections — most importantly that the store could not be encrypted
// and was written in the clear. The app sets it; the default is silent.
var Warn = func(string) {}

// DefaultDir is the directory peekerdb keeps all of its state in.
func DefaultDir() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "peekerdb"), nil
}
func DefaultPath() (string, error) {
	dir, err := DefaultDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "connections.json"), nil
}

// New returns a store bound to the default path. If the config dir is
// unavailable the store keeps an empty Path and persists fail with an
// actionable message instead of "open : no such file or directory".
func New() *Store {
	p, err := DefaultPath()
	if err != nil {
		return &Store{pathErr: err}
	}
	return &Store{Path: p}
}

// writeAtomic writes data to path via a same-directory temp file and
// os.Rename, so a crash mid-write can never truncate the real file.
// os.Rename replaces an existing destination on Windows (MoveFileEx
// with MOVEFILE_REPLACE_EXISTING), POSIX, and every other platform Go
// supports — no remove-then-rename is needed.
//
// The temp file has a random name. A predictable one could be planted
// as a symlink and followed, and a pre-existing temp file would keep
// its own permissions, which may be wider than perm.
func writeAtomic(path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	// MkdirAll leaves an existing directory as it found it, so tighten a
	// pre-existing one. Best effort: a directory owned by someone else
	// must not stop the write.
	_ = os.Chmod(dir, 0o700)
	f, err := os.CreateTemp(dir, ".peekerdb-*.tmp")
	if err != nil {
		return err
	}
	tmp := f.Name()
	defer os.Remove(tmp) // no-op once the rename succeeds
	if err := f.Chmod(perm); err != nil {
		f.Close()
		return err
	}
	if _, err := f.Write(data); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func Load() (*Store, error) {
	p, err := DefaultPath()
	if err != nil {
		return nil, err
	}
	return loadAt(p)
}

// loadAt reads the saved connections from path. A file that cannot be
// read or parsed yields a store that refuses to save, so the next write
// cannot silently destroy the connections it failed to read.
func loadAt(path string) (*Store, error) {
	s := &Store{Path: path}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return s, nil
		}
		s.loadErr = err
		return s, err
	}
	decrypted, derr := decryptPayload(data)
	if derr == nil {
		data = decrypted
	}
	var profiles []Profile
	if err := json.Unmarshal(data, &profiles); err != nil {
		s.loadErr = err
		return s, fmt.Errorf("parse %s: %w", path, err)
	}
	s.profiles = profiles
	return s, nil
}

func (s *Store) List() []Profile {
	out := append([]Profile(nil), s.profiles...)
	sort.Slice(out, func(i, j int) bool {
		return strings.ToLower(out[i].Name) < strings.ToLower(out[j].Name)
	})
	return out
}

func (s *Store) Get(name string) (Profile, bool) {
	for _, p := range s.profiles {
		if strings.EqualFold(p.Name, name) {
			return p, true
		}
	}
	return Profile{}, false
}

// Upsert adds or replaces the profile with the same (case-insensitive) name.
func (s *Store) Upsert(name, conn string) error {
	name = strings.TrimSpace(name)
	conn = strings.TrimSpace(conn)
	if name == "" {
		return fmt.Errorf("profile name is empty")
	}
	if conn == "" {
		return fmt.Errorf("connection string is empty")
	}
	now := time.Now().UTC()
	next := make([]Profile, 0, len(s.profiles)+1)
	replaced := false
	for _, p := range s.profiles {
		if !replaced && strings.EqualFold(p.Name, name) {
			next = append(next, Profile{Name: p.Name, Conn: conn, UpdatedAt: now})
			replaced = true
			continue
		}
		next = append(next, p)
	}
	if !replaced {
		next = append(next, Profile{Name: name, Conn: conn, UpdatedAt: now})
	}
	return s.persist(next)
}

func (s *Store) Delete(name string) error {
	next := make([]Profile, 0, len(s.profiles))
	found := false
	for i, p := range s.profiles {
		if strings.EqualFold(p.Name, name) {
			next = append(next, s.profiles[i+1:]...)
			found = true
			break
		}
		next = append(next, p)
	}
	if !found {
		return fmt.Errorf("no saved connection %q", name)
	}
	return s.persist(next)
}

// persist writes next and, only on success, adopts it as the store's
// profiles. On failure s.profiles is untouched, so the UI list and the
// disk file can never diverge.
func (s *Store) persist(next []Profile) error {
	if s.Path == "" {
		return fmt.Errorf("saved connections have no path (config dir unavailable): %w", s.pathErr)
	}
	if s.loadErr != nil {
		return fmt.Errorf("saved connections could not be read (%w); refusing to overwrite %s", s.loadErr, s.Path)
	}
	data, err := json.MarshalIndent(next, "", "  ")
	if err != nil {
		return err
	}
	payload, eerr := encryptPayload(append(data, '\n'))
	if eerr != nil {
		// The store holds database passwords. Writing them in the clear
		// is a downgrade the user must know about, not a detail to
		// swallow.
		payload = append(data, '\n')
		if Warn != nil {
			Warn("saved connections could not be encrypted (" + eerr.Error() +
				"); they were written in plain text to " + s.Path)
		}
	}
	if err := writeAtomic(s.Path, payload, 0o600); err != nil {
		return err
	}
	s.profiles = next
	return nil
}
