package tts

import (
	"context"
	"testing"
	"time"
)

func TestSynthesizeEdgeLive(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	audio, err := SynthesizeEdge(ctx, "hello")
	if err != nil {
		t.Fatalf("SynthesizeEdge: %v", err)
	}
	if len(audio) < 200 {
		t.Fatalf("audio too short: %d", len(audio))
	}
	// MP3: ID3 or frame sync
	if !(audio[0] == 0x49 && audio[1] == 0x44 && audio[2] == 0x33) && !(audio[0] == 0xff && (audio[1]&0xe0) == 0xe0) {
		t.Fatalf("not mp3 header: %x", audio[:8])
	}
	t.Logf("edge ok bytes=%d", len(audio))
}
