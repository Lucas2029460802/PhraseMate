package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"phrasemate/internal/models"
)

const (
	writingSelectCols     = `id, prompt, prompt_images, body, model_text, model_filename, created_at, updated_at`
	maxPromptImages       = 6
	maxPromptImageBytes   = 2 * 1024 * 1024 // decoded size per image
	maxPromptImagesTotal  = 8 * 1024 * 1024
)

func (s *Store) migrateWritingsSchema() error {
	var hasPrompt, hasTitle int
	_ = s.db.QueryRow(`SELECT COUNT(*) FROM pragma_table_info('writings') WHERE name='prompt'`).Scan(&hasPrompt)
	_ = s.db.QueryRow(`SELECT COUNT(*) FROM pragma_table_info('writings') WHERE name='title'`).Scan(&hasTitle)
	if hasPrompt == 0 && hasTitle > 0 {
		if _, err := s.db.Exec(`ALTER TABLE writings RENAME COLUMN title TO prompt`); err != nil {
			return err
		}
	} else if hasPrompt == 0 {
		if err := s.addColumn(`ALTER TABLE writings ADD COLUMN prompt TEXT NOT NULL DEFAULT ''`); err != nil {
			return err
		}
	}
	if err := s.addColumn(`ALTER TABLE writings ADD COLUMN prompt_images TEXT NOT NULL DEFAULT ''`); err != nil {
		return err
	}
	return nil
}

// ListWritings returns writing entries newest-updated first.
// Image payloads are omitted; PromptImageCount is set instead.
func (s *Store) ListWritings() ([]models.Writing, error) {
	rows, err := s.db.Query(`
SELECT ` + writingSelectCols + `
FROM writings ORDER BY datetime(updated_at) DESC, id DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []models.Writing
	for rows.Next() {
		w, err := scanWriting(rows)
		if err != nil {
			return nil, err
		}
		w.PromptImageCount = len(w.PromptImages)
		w.PromptImages = nil
		list = append(list, *w)
	}
	return list, rows.Err()
}

// GetWriting returns one writing by id, or nil when missing.
func (s *Store) GetWriting(id int64) (*models.Writing, error) {
	row := s.db.QueryRow(`
SELECT `+writingSelectCols+`
FROM writings WHERE id=?`, id)
	return scanWriting(row)
}

// CreateWriting inserts a writing entry.
func (s *Store) CreateWriting(prompt, body, modelText string, images []models.WritingImage) (*models.Writing, error) {
	now := time.Now().UTC()
	prompt = strings.TrimSpace(prompt)
	imagesJSON, err := encodePromptImages(images)
	if err != nil {
		return nil, err
	}
	res, err := s.db.Exec(`
INSERT INTO writings (prompt, prompt_images, body, model_text, model_filename, created_at, updated_at)
VALUES (?, ?, ?, ?, '', ?, ?)`,
		prompt, imagesJSON, body, modelText, now.Format(time.RFC3339), now.Format(time.RFC3339),
	)
	if err != nil {
		return nil, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return nil, err
	}
	return s.GetWriting(id)
}

// UpdateWriting saves prompt, images, body, and model text.
func (s *Store) UpdateWriting(id int64, prompt, body, modelText, modelFilename string, images []models.WritingImage) (*models.Writing, error) {
	prompt = strings.TrimSpace(prompt)
	imagesJSON, err := encodePromptImages(images)
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	res, err := s.db.Exec(`
UPDATE writings SET prompt=?, prompt_images=?, body=?, model_text=?, model_filename=?, updated_at=?
WHERE id=?`,
		prompt, imagesJSON, body, modelText, modelFilename, now.Format(time.RFC3339), id,
	)
	if err != nil {
		return nil, err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return nil, fmt.Errorf("未找到该作文")
	}
	return s.GetWriting(id)
}

// SetWritingModelFile replaces the model essay from an uploaded file.
func (s *Store) SetWritingModelFile(id int64, modelText, filename string) (*models.Writing, error) {
	existing, err := s.GetWriting(id)
	if err != nil {
		return nil, err
	}
	if existing == nil {
		return nil, fmt.Errorf("未找到该作文")
	}
	return s.UpdateWriting(id, existing.Prompt, existing.Body, modelText, strings.TrimSpace(filename), existing.PromptImages)
}

// DeleteWriting removes a writing by id.
func (s *Store) DeleteWriting(id int64) error {
	res, err := s.db.Exec(`DELETE FROM writings WHERE id = ?`, id)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return fmt.Errorf("未找到该作文")
	}
	return nil
}

func encodePromptImages(images []models.WritingImage) (string, error) {
	cleaned, err := normalizePromptImages(images)
	if err != nil {
		return "", err
	}
	if len(cleaned) == 0 {
		return "", nil
	}
	b, err := json.Marshal(cleaned)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func parsePromptImages(raw string) []models.WritingImage {
	raw = strings.TrimSpace(raw)
	if raw == "" || raw == "null" {
		return nil
	}
	var images []models.WritingImage
	if err := json.Unmarshal([]byte(raw), &images); err != nil {
		return nil
	}
	cleaned, err := normalizePromptImages(images)
	if err != nil {
		return nil
	}
	return cleaned
}

func normalizePromptImages(images []models.WritingImage) ([]models.WritingImage, error) {
	if len(images) == 0 {
		return nil, nil
	}
	out := make([]models.WritingImage, 0, len(images))
	total := 0
	for _, img := range images {
		mime := strings.ToLower(strings.TrimSpace(img.Mime))
		data := strings.TrimSpace(img.Data)
		if data == "" {
			continue
		}
		if strings.HasPrefix(data, "data:") {
			if i := strings.Index(data, ","); i >= 0 {
				header := data[:i]
				data = data[i+1:]
				if mime == "" {
					if m := strings.TrimPrefix(header, "data:"); strings.Contains(m, ";") {
						mime = strings.SplitN(m, ";", 2)[0]
					}
				}
			}
		}
		switch mime {
		case "image/png", "image/jpeg", "image/jpg", "image/webp", "image/gif":
			if mime == "image/jpg" {
				mime = "image/jpeg"
			}
		default:
			return nil, fmt.Errorf("不支持的图片格式，请使用 PNG / JPEG / WebP / GIF")
		}
		// base64 expands ~4/3; approximate decoded size
		approx := len(data) * 3 / 4
		if approx > maxPromptImageBytes {
			return nil, fmt.Errorf("单张截图不能超过 2MB，请压缩后再贴")
		}
		total += approx
		if total > maxPromptImagesTotal {
			return nil, fmt.Errorf("题目截图总大小不能超过 8MB")
		}
		out = append(out, models.WritingImage{Mime: mime, Data: data})
		if len(out) >= maxPromptImages {
			break
		}
	}
	return out, nil
}

func scanWriting(row scanner) (*models.Writing, error) {
	var w models.Writing
	var created, updated, imagesJSON string
	err := row.Scan(&w.ID, &w.Prompt, &imagesJSON, &w.Body, &w.ModelText, &w.ModelFilename, &created, &updated)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	w.PromptImages = parsePromptImages(imagesJSON)
	w.PromptImageCount = len(w.PromptImages)
	w.CreatedAt, _ = time.Parse(time.RFC3339, created)
	w.UpdatedAt, _ = time.Parse(time.RFC3339, updated)
	return &w, nil
}
