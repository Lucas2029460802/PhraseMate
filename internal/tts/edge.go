package tts

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/xml"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

const (
	trustedClientToken = "6A5AA1D4EAFF4E9FB37E23D68491D6F4"
	chromiumFull       = "143.0.3650.75"
	defaultVoice       = "en-US-EmmaMultilingualNeural"
	winEpochSeconds    = 11644473600
)

// SynthesizeEdge returns MP3 audio from Microsoft Edge online TTS.
func SynthesizeEdge(ctx context.Context, text string) ([]byte, error) {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil, fmt.Errorf("empty text")
	}
	text = sanitizeTTSText(text)

	reqID := strings.ReplaceAll(uuid.NewString(), "-", "")
	gec := generateSecMSGEC(time.Now().UTC())
	q := url.Values{}
	q.Set("TrustedClientToken", trustedClientToken)
	q.Set("ConnectionId", reqID)
	q.Set("Sec-MS-GEC", gec)
	q.Set("Sec-MS-GEC-Version", "1-"+chromiumFull)
	wsURL := "wss://speech.platform.bing.com/consumer/speech/synthesize/readaloud/edge/v1?" + q.Encode()

	hdr := http.Header{}
	hdr.Set("Pragma", "no-cache")
	hdr.Set("Cache-Control", "no-cache")
	hdr.Set("Origin", "chrome-extension://jdiccldimpdaibmpdkjnbmckianbfold")
	hdr.Set("User-Agent", edgeUserAgent())
	hdr.Set("Accept-Language", "en-US,en;q=0.9")
	hdr.Set("Cookie", "muid="+strings.ToUpper(strings.ReplaceAll(uuid.NewString(), "-", ""))+";")

	dialer := websocket.Dialer{
		HandshakeTimeout: 8 * time.Second,
	}
	conn, resp, err := dialer.DialContext(ctx, wsURL, hdr)
	if err != nil {
		if resp != nil && resp.StatusCode == http.StatusForbidden {
			// Clock skew: regenerate token using server Date and retry once.
			if t := parseHTTPDate(resp.Header.Get("Date")); !t.IsZero() {
				gec = generateSecMSGEC(t)
				q.Set("Sec-MS-GEC", gec)
				q.Set("ConnectionId", strings.ReplaceAll(uuid.NewString(), "-", ""))
				wsURL = "wss://speech.platform.bing.com/consumer/speech/synthesize/readaloud/edge/v1?" + q.Encode()
				conn, resp, err = dialer.DialContext(ctx, wsURL, hdr)
			}
		}
		if err != nil {
			status := 0
			if resp != nil {
				status = resp.StatusCode
			}
			return nil, fmt.Errorf("edge dial: %w (http=%d)", err, status)
		}
	}
	defer conn.Close()
	_ = conn.SetReadDeadline(deadlineFrom(ctx, 20*time.Second))
	_ = conn.SetWriteDeadline(deadlineFrom(ctx, 10*time.Second))

	configMsg := fmt.Sprintf(
		"X-Timestamp:%s\r\nContent-Type:application/json; charset=utf-8\r\nPath:speech.config\r\n\r\n"+
			`{"context":{"synthesis":{"audio":{"metadataoptions":{"sentenceBoundaryEnabled":"false","wordBoundaryEnabled":"true"},"outputFormat":"audio-24khz-48kbitrate-mono-mp3"}}}}`+"\r\n",
		jsDateString(),
	)
	if err := conn.WriteMessage(websocket.TextMessage, []byte(configMsg)); err != nil {
		return nil, fmt.Errorf("edge config: %w", err)
	}

	ssml := buildSSML(defaultVoice, text)
	ssmlMsg := fmt.Sprintf(
		"X-RequestId:%s\r\nContent-Type:application/ssml+xml\r\nX-Timestamp:%sZ\r\nPath:ssml\r\n\r\n%s",
		reqID, jsDateString(), ssml,
	)
	if err := conn.WriteMessage(websocket.TextMessage, []byte(ssmlMsg)); err != nil {
		return nil, fmt.Errorf("edge ssml: %w", err)
	}

	var audio []byte
	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}
		msgType, data, err := conn.ReadMessage()
		if err != nil {
			if len(audio) > 0 {
				break
			}
			return nil, fmt.Errorf("edge read: %w", err)
		}
		switch msgType {
		case websocket.TextMessage:
			path := extractPath(data)
			if path == "turn.end" {
				if len(audio) == 0 {
					return nil, fmt.Errorf("edge: no audio")
				}
				return audio, nil
			}
		case websocket.BinaryMessage:
			chunk, ok := extractAudioChunk(data)
			if ok && len(chunk) > 0 {
				audio = append(audio, chunk...)
			}
		}
	}
	if len(audio) == 0 {
		return nil, fmt.Errorf("edge: no audio")
	}
	return audio, nil
}

func edgeUserAgent() string {
	major := strings.SplitN(chromiumFull, ".", 2)[0]
	return fmt.Sprintf(
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/%s.0.0.0 Safari/537.36 Edg/%s.0.0.0",
		major, major,
	)
}

func generateSecMSGEC(now time.Time) string {
	ticks := float64(now.Unix()) + float64(now.Nanosecond())/1e9
	ticks += winEpochSeconds
	ticks -= float64(int64(ticks) % 300)
	ticks *= 1e9 / 100 // 100-ns intervals
	payload := fmt.Sprintf("%.0f%s", ticks, trustedClientToken)
	sum := sha256.Sum256([]byte(payload))
	return strings.ToUpper(hex.EncodeToString(sum[:]))
}

func buildSSML(voice, text string) string {
	type prosody struct {
		Pitch  string `xml:"pitch,attr"`
		Rate   string `xml:"rate,attr"`
		Volume string `xml:"volume,attr"`
		Text   string `xml:",chardata"`
	}
	type voiceEl struct {
		Name    string  `xml:"name,attr"`
		Prosody prosody `xml:"prosody"`
	}
	type speak struct {
		XMLName xml.Name `xml:"speak"`
		Version string   `xml:"version,attr"`
		Xmlns   string   `xml:"xmlns,attr"`
		Lang    string   `xml:"xml:lang,attr"`
		Voice   voiceEl  `xml:"voice"`
	}
	doc := speak{
		Version: "1.0",
		Xmlns:   "http://www.w3.org/2001/10/synthesis",
		Lang:    "en-US",
		Voice: voiceEl{
			Name: voice,
			Prosody: prosody{
				Pitch:  "+0Hz",
				Rate:   "+0%",
				Volume: "+0%",
				Text:   text,
			},
		},
	}
	out, err := xml.Marshal(doc)
	if err != nil {
		escaped := xmlEscape(text)
		return fmt.Sprintf(
			`<speak version="1.0" xmlns="http://www.w3.org/2001/10/synthesis" xml:lang="en-US"><voice name="%s"><prosody pitch="+0Hz" rate="+0%%" volume="+0%%">%s</prosody></voice></speak>`,
			voice, escaped,
		)
	}
	return string(out)
}

func xmlEscape(s string) string {
	var b strings.Builder
	_ = xml.EscapeText(&b, []byte(s))
	return b.String()
}

func sanitizeTTSText(s string) string {
	runes := []rune(s)
	for i, r := range runes {
		c := int(r)
		if (0 <= c && c <= 8) || (11 <= c && c <= 12) || (14 <= c && c <= 31) {
			runes[i] = ' '
		}
	}
	return string(runes)
}

func extractPath(msg []byte) string {
	idx := indexOf(msg, []byte("\r\n\r\n"))
	header := msg
	if idx >= 0 {
		header = msg[:idx]
	}
	for _, line := range strings.Split(string(header), "\r\n") {
		if strings.HasPrefix(strings.ToLower(line), "path:") {
			return strings.TrimSpace(line[5:])
		}
	}
	return ""
}

func extractAudioChunk(msg []byte) ([]byte, bool) {
	if len(msg) < 2 {
		return nil, false
	}
	headerLen := int(msg[0])<<8 | int(msg[1])
	if headerLen+2 > len(msg) {
		return nil, false
	}
	headers := msg[2 : 2+headerLen]
	data := msg[2+headerLen:]
	if extractPath(headers) != "audio" {
		return nil, false
	}
	// Trailing empty audio frame is normal.
	return data, true
}

func indexOf(b, sep []byte) int {
	return strings.Index(string(b), string(sep))
}

func jsDateString() string {
	return time.Now().UTC().Format("Mon Jan 02 2006 15:04:05 GMT+0000 (Coordinated Universal Time)")
}

func parseHTTPDate(s string) time.Time {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}
	}
	t, err := http.ParseTime(s)
	if err != nil {
		return time.Time{}
	}
	return t
}

func deadlineFrom(ctx context.Context, fallback time.Duration) time.Time {
	if dl, ok := ctx.Deadline(); ok {
		return dl
	}
	return time.Now().Add(fallback)
}
