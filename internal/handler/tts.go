package handler

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
	"unicode/utf8"

	"phrasemate/internal/dict"
	"phrasemate/internal/tts"
)

const (
	ttsTimeout   = 15 * time.Second
	ttsMaxRunes  = 180
	ttsUserAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/122.0.0.0 Safari/537.36"
)

func (a *API) handleTTS(w http.ResponseWriter, r *http.Request) {
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	if q == "" {
		http.Error(w, "missing q", http.StatusBadRequest)
		return
	}
	q = truncateRunes(q, ttsMaxRunes)

	ctx, cancel := context.WithTimeout(r.Context(), ttsTimeout)
	defer cancel()

	// Prefer an explicit dictionary audio URL when allowlisted.
	if src := dict.NormalizeAudioURL(r.URL.Query().Get("src")); src != "" && isAllowedAudioURL(src) {
		if err := proxyAudio(ctx, w, src); err == nil {
			return
		}
	}

	// Single words: try Free Dictionary pronunciation first.
	if a.dict != nil && dict.IsSingleWord(q) {
		if audio, err := a.dict.LookupAudio(ctx, q); err == nil && audio != "" {
			if err := proxyAudio(ctx, w, audio); err == nil {
				return
			}
		}
	}

	// Edge neural voice (replaces Google TTS).
	if err := proxyEdgeTTS(ctx, w, q); err == nil {
		return
	}

	// Youdao voice works well in mainland China as a fallback.
	if err := proxyYoudaoTTS(ctx, w, q); err != nil {
		http.Error(w, "tts unavailable", http.StatusBadGateway)
	}
}

func proxyEdgeTTS(ctx context.Context, w http.ResponseWriter, text string) error {
	audio, err := tts.SynthesizeEdge(ctx, text)
	if err != nil {
		return err
	}
	if len(audio) < 64 {
		return fmt.Errorf("edge tts empty")
	}
	w.Header().Set("Content-Type", "audio/mpeg")
	w.Header().Set("Cache-Control", "public, max-age=86400")
	_, err = w.Write(audio)
	return err
}

func proxyYoudaoTTS(ctx context.Context, w http.ResponseWriter, text string) error {
	// type=2 → US English
	u := "https://dict.youdao.com/dictvoice?type=2&audio=" + url.QueryEscape(text)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", ttsUserAgent)
	req.Header.Set("Referer", "https://www.youdao.com/")
	req.Header.Set("Accept", "*/*")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("youdao tts HTTP %d", resp.StatusCode)
	}
	ct := resp.Header.Get("Content-Type")
	if ct == "" || strings.HasPrefix(ct, "text/") {
		ct = "audio/mpeg"
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		return err
	}
	if len(body) < 64 {
		return fmt.Errorf("youdao tts empty")
	}
	w.Header().Set("Content-Type", ct)
	w.Header().Set("Cache-Control", "public, max-age=86400")
	_, err = w.Write(body)
	return err
}

func proxyAudio(ctx context.Context, w http.ResponseWriter, audioURL string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, audioURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", ttsUserAgent)
	req.Header.Set("Accept", "*/*")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("audio HTTP %d", resp.StatusCode)
	}
	ct := resp.Header.Get("Content-Type")
	if ct == "" || strings.HasPrefix(ct, "text/") {
		ct = "audio/mpeg"
	}
	w.Header().Set("Content-Type", ct)
	w.Header().Set("Cache-Control", "public, max-age=86400")
	_, err = io.Copy(w, resp.Body)
	return err
}

func isAllowedAudioURL(u string) bool {
	parsed, err := url.Parse(u)
	if err != nil || parsed.Scheme != "https" {
		return false
	}
	host := strings.ToLower(parsed.Hostname())
	switch host {
	case "ssl.gstatic.com", "gstatic.com", "api.dictionaryapi.dev", "dict.youdao.com":
		return true
	}
	return strings.HasSuffix(host, ".gstatic.com") || strings.HasSuffix(host, ".dictionaryapi.dev")
}

func truncateRunes(s string, max int) string {
	if max <= 0 || utf8.RuneCountInString(s) <= max {
		return s
	}
	runes := []rune(s)
	return string(runes[:max])
}
