package models

import "testing"

func TestNormalizeForms(t *testing.T) {
	got := NormalizeForms("beautiful", []RelatedForm{
		{Word: "beautiful", POS: "adj.", MeaningZH: "美丽的"},
		{Word: "beauty", PartOfSpeech: "名词", Meaning: "美"},
		{Word: "beauty", POS: "n.", MeaningZH: "重复"},
		{Word: "beautify", POS: "动词", MeaningZH: "美化"},
		{Word: "beautifully", POS: "adv.", MeaningZH: "美丽地"},
		{Word: "beautician", POS: "n.", MeaningZH: "美容师"},
		{Word: "   ", POS: "n."},
	})
	if len(got) != 4 {
		t.Fatalf("len=%d %+v", len(got), got)
	}
	if got[0].Word != "beauty" || got[0].POS != "n." || got[0].MeaningZH != "美" {
		t.Fatalf("first=%+v", got[0])
	}
	if got[1].Word != "beautify" || got[1].POS != "v." {
		t.Fatalf("second=%+v", got[1])
	}
	if got[0].Term != "" || got[0].Meaning != "" || got[0].PartOfSpeech != "" {
		t.Fatalf("aliases leaked: %+v", got[0])
	}
}

func TestNormalizePhrasesSkipsHeadword(t *testing.T) {
	got := NormalizePhrases("happy", []Phrase{
		{Text: "happy", MeaningZH: "快乐的"},
		{Alt: "happy with", Meaning: "对……满意"},
		{Text: "happy with", MeaningZH: "重复"},
		{Text: "happy birthday", MeaningZH: "生日快乐"},
	})
	if len(got) != 2 {
		t.Fatalf("len=%d %+v", len(got), got)
	}
	if got[0].Text != "happy with" || got[0].MeaningZH != "对……满意" {
		t.Fatalf("first=%+v", got[0])
	}
}

func TestEncodeSliceNilVsEmpty(t *testing.T) {
	if EncodeSlice[RelatedForm](nil) != "" {
		t.Fatal("nil should stay blank")
	}
	if EncodeSlice([]RelatedForm{}) != "[]" {
		t.Fatal("empty list should be []")
	}
	if ParseSlice[RelatedForm]("") != nil {
		t.Fatal("blank should parse as nil")
	}
	if ParseSlice[RelatedForm]("[]") == nil {
		t.Fatal("[] should parse as an empty slice")
	}
}
