package dict

import "testing"

func TestNormalizeAudioURL(t *testing.T) {
	if got := NormalizeAudioURL("//ssl.gstatic.com/a.mp3"); got != "https://ssl.gstatic.com/a.mp3" {
		t.Fatalf("got %q", got)
	}
	if got := NormalizeAudioURL(" https://example.com/a.mp3 "); got != "https://example.com/a.mp3" {
		t.Fatalf("got %q", got)
	}
	if got := NormalizeAudioURL(""); got != "" {
		t.Fatalf("got %q", got)
	}
}

func TestPickAudioPrefersUS(t *testing.T) {
	entry := apiEntry{
		Phonetics: []struct {
			Text  string `json:"text"`
			Audio string `json:"audio"`
		}{
			{Text: "/həˈləʊ/", Audio: "https://api.dictionaryapi.dev/media/pronunciations/en/hello-uk.mp3"},
			{Text: "/həˈloʊ/", Audio: "https://api.dictionaryapi.dev/media/pronunciations/en/hello-us.mp3"},
			{Text: "", Audio: "https://api.dictionaryapi.dev/media/pronunciations/en/hello-au.mp3"},
		},
	}
	got := pickAudio(entry)
	if got != "https://api.dictionaryapi.dev/media/pronunciations/en/hello-us.mp3" {
		t.Fatalf("got %q", got)
	}
}
