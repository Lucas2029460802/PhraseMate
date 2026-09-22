package store

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"phrasemate/internal/format"
	"phrasemate/internal/models"

	_ "modernc.org/sqlite"
)

// Store wraps SQLite persistence for the vocabulary notebook.
// Word rows are the working copy; gitdata publishes them to the data branch.
type Store struct {
	db       *sql.DB
	mu       sync.RWMutex
	onChange func()
	syncRef  string
	syncErr  string
}

// Open creates or opens the database at path.
func Open(path string) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("创建数据目录失败: %w", err)
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	s := &Store{db: db}
	if err := s.migrate(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return s, nil
}

func (s *Store) migrate() error {
	_, err := s.db.Exec(`
CREATE TABLE IF NOT EXISTS words (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  term TEXT NOT NULL COLLATE NOCASE,
  phonetic TEXT NOT NULL DEFAULT '',
  meaning_en TEXT NOT NULL DEFAULT '',
  meaning_zh TEXT NOT NULL DEFAULT '',
  example_en TEXT NOT NULL DEFAULT '',
  example_zh TEXT NOT NULL DEFAULT '',
  part_of_speech TEXT NOT NULL DEFAULT '',
  status TEXT NOT NULL DEFAULT 'ready',
  error_msg TEXT NOT NULL DEFAULT '',
  created_at TEXT NOT NULL
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_words_term ON words(term);
CREATE TABLE IF NOT EXISTS settings (
  key TEXT PRIMARY KEY,
  value TEXT NOT NULL DEFAULT ''
);
`)
	if err != nil {
		return err
	}
	// Soft migrations for older DBs.
	_, _ = s.db.Exec(`ALTER TABLE words ADD COLUMN status TEXT NOT NULL DEFAULT 'ready'`)
	_, _ = s.db.Exec(`ALTER TABLE words ADD COLUMN error_msg TEXT NOT NULL DEFAULT ''`)
	_, _ = s.db.Exec(`UPDATE words SET status='ready' WHERE IFNULL(status,'')=''`)
	_, _ = s.db.Exec(`UPDATE words SET status='pending' WHERE status='ready' AND TRIM(meaning_zh)='' AND TRIM(meaning_en)=''`)
	_, _ = s.db.Exec(`UPDATE words SET status='pending' WHERE status='ready' AND TRIM(meaning_zh)='' AND TRIM(meaning_en)<>''`)
	if err := s.normalizeStoredPhonetics(); err != nil {
		return err
	}
	return nil
}

func (s *Store) normalizeStoredPhonetics() error {
	rows, err := s.db.Query(`SELECT id, phonetic FROM words WHERE TRIM(phonetic) <> ''`)
	if err != nil {
		return err
	}
	type phoneticUpdate struct {
		id       int64
		phonetic string
	}
	var updates []phoneticUpdate
	for rows.Next() {
		var id int64
		var phonetic string
		if err := rows.Scan(&id, &phonetic); err != nil {
			_ = rows.Close()
			return err
		}
		norm := format.NormalizePhonetic(phonetic)
		if norm == phonetic {
			continue
		}
		updates = append(updates, phoneticUpdate{id: id, phonetic: norm})
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return err
	}
	if err := rows.Close(); err != nil {
		return err
	}
	// MaxOpenConns is 1, so updates must wait until the read cursor is closed.
	for _, upd := range updates {
		if _, err := s.db.Exec(`UPDATE words SET phonetic=? WHERE id=?`, upd.phonetic, upd.id); err != nil {
			return err
		}
	}
	return nil
}

const (
	SettingAPIKey  = "api_key"
	SettingBaseURL = "base_url"
	SettingModel   = "model"
)

// GetSetting returns a settings value (empty if missing).
func (s *Store) GetSetting(key string) (string, error) {
	var val string
	err := s.db.QueryRow(`SELECT value FROM settings WHERE key=?`, key).Scan(&val)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return val, err
}

// SetSetting upserts a settings value.
func (s *Store) SetSetting(key, value string) error {
	_, err := s.db.Exec(`
INSERT INTO settings (key, value) VALUES (?, ?)
ON CONFLICT(key) DO UPDATE SET value=excluded.value`, key, value)
	return err
}

// DeleteSetting removes a settings key.
func (s *Store) DeleteSetting(key string) error {
	_, err := s.db.Exec(`DELETE FROM settings WHERE key=?`, key)
	return err
}

// LoadAppSettings reads API credentials from the settings table.
func (s *Store) LoadAppSettings() (apiKey, baseURL, model string, err error) {
	apiKey, err = s.GetSetting(SettingAPIKey)
	if err != nil {
		return "", "", "", err
	}
	baseURL, err = s.GetSetting(SettingBaseURL)
	if err != nil {
		return "", "", "", err
	}
	model, err = s.GetSetting(SettingModel)
	if err != nil {
		return "", "", "", err
	}
	return apiKey, baseURL, model, nil
}

// Close closes the database.
func (s *Store) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

// SetAfterWordChange runs after a vocabulary row is inserted, updated, or deleted.
func (s *Store) SetAfterWordChange(fn func()) {
	if s == nil {
		return
	}
	s.mu.Lock()
	s.onChange = fn
	s.mu.Unlock()
}

// SetSyncState records whether the data branch push succeeded.
func (s *Store) SetSyncState(branch, errMsg string) {
	if s == nil {
		return
	}
	s.mu.Lock()
	s.syncRef = branch
	s.syncErr = errMsg
	s.mu.Unlock()
}

// SyncState returns the data branch name and the latest push error.
func (s *Store) SyncState() (string, string) {
	if s == nil {
		return "", ""
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.syncRef, s.syncErr
}

func (s *Store) changed() {
	if s == nil {
		return
	}
	s.mu.RLock()
	fn := s.onChange
	s.mu.RUnlock()
	if fn != nil {
		fn()
	}
}

// ReplaceAllWords replaces the vocabulary table with the given snapshot.
// It does not fire the change hook.
func (s *Store) ReplaceAllWords(words []models.Word) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`DELETE FROM words`); err != nil {
		return err
	}
	var maxID int64
	for _, w := range words {
		term := strings.TrimSpace(w.Term)
		if term == "" {
			continue
		}
		if w.ID > maxID {
			maxID = w.ID
		}
		status := strings.TrimSpace(w.Status)
		if status == "" {
			status = models.StatusReady
		}
		created := w.CreatedAt.UTC().Truncate(time.Second)
		if w.CreatedAt.IsZero() {
			created = time.Unix(0, 0).UTC()
		}
		if _, err := tx.Exec(`
INSERT INTO words (id, term, phonetic, meaning_en, meaning_zh, example_en, example_zh, part_of_speech, status, error_msg, created_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			w.ID, term, format.NormalizePhonetic(w.Phonetic), w.MeaningEN, w.MeaningZH,
			w.ExampleEN, w.ExampleZH, w.PartOfSpeech, status, w.ErrorMsg, created.Format(time.RFC3339),
		); err != nil {
			return err
		}
	}
	if maxID > 0 {
		var seqTable string
		err := tx.QueryRow(`SELECT name FROM sqlite_master WHERE type='table' AND name='sqlite_sequence'`).Scan(&seqTable)
		if err == nil {
			res, err := tx.Exec(`UPDATE sqlite_sequence SET seq=? WHERE name='words'`, maxID)
			if err != nil {
				return err
			}
			n, _ := res.RowsAffected()
			if n == 0 {
				if _, err := tx.Exec(`INSERT INTO sqlite_sequence(name, seq) VALUES ('words', ?)`, maxID); err != nil {
					return err
				}
			}
		}
	}
	return tx.Commit()
}

// Capture inserts a term immediately as pending, or bumps an existing one.
func (s *Store) Capture(term string) (*models.Word, bool, error) {
	term = strings.TrimSpace(term)
	if term == "" {
		return nil, false, fmt.Errorf("生词不能为空")
	}
	now := time.Now().UTC()
	existing, err := s.FindByTerm(term)
	if err != nil {
		return nil, false, err
	}
	if existing != nil {
		if existing.Status == models.StatusReady && strings.TrimSpace(existing.MeaningZH) != "" {
			// Already explained — just bump to top.
			_, err = s.db.Exec(`UPDATE words SET created_at=?, error_msg='' WHERE id=?`, now.Format(time.RFC3339), existing.ID)
			if err != nil {
				return nil, false, err
			}
			existing.CreatedAt = now
			s.changed()
			return existing, false, nil
		}
		// Still pending/error — requeue.
		_, err = s.db.Exec(`UPDATE words SET status=?, error_msg='', created_at=? WHERE id=?`,
			models.StatusPending, now.Format(time.RFC3339), existing.ID)
		if err != nil {
			return nil, false, err
		}
		existing.Status = models.StatusPending
		existing.ErrorMsg = ""
		existing.CreatedAt = now
		s.changed()
		return existing, true, nil
	}

	res, err := s.db.Exec(`
INSERT INTO words (term, phonetic, meaning_en, meaning_zh, example_en, example_zh, part_of_speech, status, error_msg, created_at)
VALUES (?, '', '', '', '', '', '', ?, '', ?)`,
		term, models.StatusPending, now.Format(time.RFC3339),
	)
	if err != nil {
		return nil, false, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return nil, false, err
	}
	s.changed()
	return &models.Word{
		ID:        id,
		Term:      term,
		Status:    models.StatusPending,
		CreatedAt: now,
	}, true, nil
}

// ApplyExplanation fills AI fields and marks ready.
func (s *Store) ApplyExplanation(id int64, exp *models.AIExplanation) (*models.Word, error) {
	exp.Phonetic = format.NormalizePhonetic(exp.Phonetic)
	_, err := s.db.Exec(`
UPDATE words SET
  term=?, phonetic=?, meaning_en=?, meaning_zh=?, example_en=?, example_zh=?,
  part_of_speech=?, status=?, error_msg=''
WHERE id=?`,
		exp.Term, exp.Phonetic, exp.MeaningEN, exp.MeaningZH, exp.ExampleEN, exp.ExampleZH,
		exp.PartOfSpeech, models.StatusReady, id,
	)
	if err != nil {
		return nil, err
	}
	s.changed()
	return s.GetByID(id)
}

// MarkError marks a pending word as failed.
func (s *Store) MarkError(id int64, msg string) error {
	_, err := s.db.Exec(`UPDATE words SET status=?, error_msg=? WHERE id=?`, models.StatusError, msg, id)
	if err != nil {
		return err
	}
	s.changed()
	return nil
}

// NextPending returns the oldest pending word, if any.
func (s *Store) NextPending() (*models.Word, error) {
	row := s.db.QueryRow(`
SELECT id, term, phonetic, meaning_en, meaning_zh, example_en, example_zh, part_of_speech, status, error_msg, created_at
FROM words WHERE status=? ORDER BY datetime(created_at) ASC, id ASC LIMIT 1`, models.StatusPending)
	return scanWord(row)
}

// CountPending returns pending count.
func (s *Store) CountPending() (int, error) {
	var n int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM words WHERE status=?`, models.StatusPending).Scan(&n)
	return n, err
}

// GetByID returns one word.
func (s *Store) GetByID(id int64) (*models.Word, error) {
	row := s.db.QueryRow(`
SELECT id, term, phonetic, meaning_en, meaning_zh, example_en, example_zh, part_of_speech, status, error_msg, created_at
FROM words WHERE id=?`, id)
	return scanWord(row)
}

// FindByTerm returns a word if present.
func (s *Store) FindByTerm(term string) (*models.Word, error) {
	row := s.db.QueryRow(`
SELECT id, term, phonetic, meaning_en, meaning_zh, example_en, example_zh, part_of_speech, status, error_msg, created_at
FROM words WHERE term = ? COLLATE NOCASE`, strings.TrimSpace(term))
	return scanWord(row)
}

// List returns all words newest first.
func (s *Store) List() ([]models.Word, error) {
	rows, err := s.db.Query(`
SELECT id, term, phonetic, meaning_en, meaning_zh, example_en, example_zh, part_of_speech, status, error_msg, created_at
FROM words ORDER BY datetime(created_at) DESC, id DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []models.Word
	for rows.Next() {
		w, err := scanWordRows(rows)
		if err != nil {
			return nil, err
		}
		list = append(list, *w)
	}
	return list, rows.Err()
}

// ListReady returns explained words for quizzes.
func (s *Store) ListReady() ([]models.Word, error) {
	rows, err := s.db.Query(`
SELECT id, term, phonetic, meaning_en, meaning_zh, example_en, example_zh, part_of_speech, status, error_msg, created_at
FROM words WHERE status=? AND TRIM(meaning_zh)<>'' ORDER BY datetime(created_at) DESC, id DESC`, models.StatusReady)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []models.Word
	for rows.Next() {
		w, err := scanWordRows(rows)
		if err != nil {
			return nil, err
		}
		list = append(list, *w)
	}
	return list, rows.Err()
}

// Count returns total vocabulary size.
func (s *Store) Count() (int, error) {
	var n int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM words`).Scan(&n)
	return n, err
}

// Delete removes a word by id.
func (s *Store) Delete(id int64) error {
	res, err := s.db.Exec(`DELETE FROM words WHERE id = ?`, id)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return fmt.Errorf("未找到该生词")
	}
	s.changed()
	return nil
}

// Retry sets a word back to pending.
func (s *Store) Retry(id int64) error {
	res, err := s.db.Exec(`UPDATE words SET status=?, error_msg='' WHERE id=?`, models.StatusPending, id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("未找到该生词")
	}
	s.changed()
	return nil
}

type scanner interface {
	Scan(dest ...any) error
}

func scanWord(row scanner) (*models.Word, error) {
	var w models.Word
	var created string
	err := row.Scan(
		&w.ID, &w.Term, &w.Phonetic, &w.MeaningEN, &w.MeaningZH, &w.ExampleEN, &w.ExampleZH,
		&w.PartOfSpeech, &w.Status, &w.ErrorMsg, &created,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if w.Status == "" {
		w.Status = models.StatusReady
	}
	w.CreatedAt, _ = time.Parse(time.RFC3339, created)
	return &w, nil
}

func scanWordRows(rows *sql.Rows) (*models.Word, error) {
	return scanWord(rows)
}
