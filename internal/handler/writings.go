package handler

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"unicode/utf8"

	"phrasemate/internal/models"
)

var errInvalidUTF8 = errors.New("文件不是有效的 UTF-8 文本，请另存为 UTF-8 后再上传")

const maxModelFileBytes = 512 * 1024

func (a *API) handleListWritings(w http.ResponseWriter, r *http.Request) {
	list, err := a.store.ListWritings()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	if list == nil {
		list = []models.Writing{}
	}
	writeJSON(w, http.StatusOK, list)
}

func (a *API) handleCreateWriting(w http.ResponseWriter, r *http.Request) {
	var req models.WritingCreateRequest
	_ = json.NewDecoder(r.Body).Decode(&req)
	writing, err := a.store.CreateWriting(req.Prompt, req.Body, req.ModelText, req.PromptImages)
	if err != nil {
		writeWritingStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, writing)
}

func (a *API) handleGetWriting(w http.ResponseWriter, r *http.Request) {
	id, ok := parseWritingID(w, r)
	if !ok {
		return
	}
	writing, err := a.store.GetWriting(id)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	if writing == nil {
		writeErr(w, http.StatusNotFound, "未找到该作文")
		return
	}
	writeJSON(w, http.StatusOK, writing)
}

func (a *API) handleUpdateWriting(w http.ResponseWriter, r *http.Request) {
	id, ok := parseWritingID(w, r)
	if !ok {
		return
	}
	var req models.WritingUpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "请求格式无效")
		return
	}
	existing, err := a.store.GetWriting(id)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	if existing == nil {
		writeErr(w, http.StatusNotFound, "未找到该作文")
		return
	}
	filename := existing.ModelFilename
	if req.ModelText != existing.ModelText {
		filename = ""
	}
	writing, err := a.store.UpdateWriting(id, req.Prompt, req.Body, req.ModelText, filename, req.PromptImages)
	if err != nil {
		writeWritingStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, writing)
}

func writeWritingStoreErr(w http.ResponseWriter, err error) {
	msg := err.Error()
	if strings.Contains(msg, "截图") || strings.Contains(msg, "图片") {
		writeErr(w, http.StatusBadRequest, msg)
		return
	}
	writeErr(w, http.StatusInternalServerError, msg)
}

func (a *API) handleDeleteWriting(w http.ResponseWriter, r *http.Request) {
	id, ok := parseWritingID(w, r)
	if !ok {
		return
	}
	if err := a.store.DeleteWriting(id); err != nil {
		writeErr(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (a *API) handleUploadWritingModel(w http.ResponseWriter, r *http.Request) {
	id, ok := parseWritingID(w, r)
	if !ok {
		return
	}
	if err := r.ParseMultipartForm(maxModelFileBytes + 64*1024); err != nil {
		writeErr(w, http.StatusBadRequest, "上传失败：表单无效或文件过大")
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		writeErr(w, http.StatusBadRequest, "请选择要上传的文件")
		return
	}
	defer file.Close()

	name := filepath.Base(header.Filename)
	ext := strings.ToLower(filepath.Ext(name))
	switch ext {
	case ".txt", ".md", ".text":
	default:
		writeErr(w, http.StatusBadRequest, "仅支持 .txt / .md / .text 文本文件")
		return
	}
	if header.Size > maxModelFileBytes {
		writeErr(w, http.StatusBadRequest, "文件不能超过 512KB")
		return
	}

	limited := io.LimitReader(file, maxModelFileBytes+1)
	raw, err := io.ReadAll(limited)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "读取文件失败")
		return
	}
	if len(raw) > maxModelFileBytes {
		writeErr(w, http.StatusBadRequest, "文件不能超过 512KB")
		return
	}
	text, err := decodeUTF8Text(raw)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}

	writing, err := a.store.SetWritingModelFile(id, text, name)
	if err != nil {
		if strings.Contains(err.Error(), "未找到") {
			writeErr(w, http.StatusNotFound, err.Error())
			return
		}
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, writing)
}

func parseWritingID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		writeErr(w, http.StatusBadRequest, "无效的作文 ID")
		return 0, false
	}
	return id, true
}

func decodeUTF8Text(raw []byte) (string, error) {
	raw = stripUTF8BOM(raw)
	if !utf8.Valid(raw) {
		return "", errInvalidUTF8
	}
	return string(raw), nil
}

func stripUTF8BOM(b []byte) []byte {
	if len(b) >= 3 && b[0] == 0xEF && b[1] == 0xBB && b[2] == 0xBF {
		return b[3:]
	}
	return b
}
