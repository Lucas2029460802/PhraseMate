// Package quiz builds multiple-choice practice items from the notebook.
package quiz

import (
	"fmt"
	"math/rand"
	"strings"
	"time"

	"phrasemate/internal/models"
)

var questionTypes = []string{
	models.QuizWordToDef,
	models.QuizDefToWord,
	models.QuizClosestMeaning,
	models.QuizZhToWord,
	models.QuizWordToZh,
}

// Build creates up to count quiz questions from ready notebook words.
// Words should already be priority-sorted (untested / high-wrong first).
// Stems are built locally so the target word/definition is never left blank.
func Build(words []models.Word, count int) ([]models.QuizQuestion, error) {
	usable := filterUsable(words)
	if len(usable) < 2 {
		return nil, fmt.Errorf("至少需要 2 个已生成释义的生词才能出题")
	}
	if count <= 0 {
		count = 5
	}
	if count > 10 {
		count = 10
	}
	if count > len(usable) {
		count = len(usable)
	}

	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	selected := pickWeighted(usable, count, rng)

	out := make([]models.QuizQuestion, 0, len(selected))
	for i, target := range selected {
		typ := questionTypes[i%len(questionTypes)]
		q, ok := buildOne(target, usable, typ, rng)
		if !ok {
			// Fall back through other types if this target lacks EN/ZH fields.
			for _, alt := range questionTypes {
				if alt == typ {
					continue
				}
				if q2, ok2 := buildOne(target, usable, alt, rng); ok2 {
					q, ok = q2, true
					break
				}
			}
		}
		if ok {
			out = append(out, q)
		}
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("未能生成测验题目，请确认生词已有中英文释义")
	}
	return out, nil
}

func filterUsable(words []models.Word) []models.Word {
	out := make([]models.Word, 0, len(words))
	for _, w := range words {
		if strings.TrimSpace(w.Term) == "" {
			continue
		}
		if strings.TrimSpace(w.MeaningZH) == "" && strings.TrimSpace(w.MeaningEN) == "" {
			continue
		}
		out = append(out, w)
	}
	return out
}

// pickWeighted prefers never-tested words, then high wrong counts.
func pickWeighted(words []models.Word, count int, rng *rand.Rand) []models.Word {
	if count >= len(words) {
		cp := append([]models.Word(nil), words...)
		rng.Shuffle(len(cp), func(i, j int) { cp[i], cp[j] = cp[j], cp[i] })
		return cp
	}

	var untested, tested []models.Word
	for _, w := range words {
		if w.QuizTested <= 0 {
			untested = append(untested, w)
		} else {
			tested = append(tested, w)
		}
	}
	rng.Shuffle(len(untested), func(i, j int) { untested[i], untested[j] = untested[j], untested[i] })

	picked := make([]models.Word, 0, count)
	picked = append(picked, untested...)
	if len(picked) > count {
		picked = picked[:count]
		return picked
	}

	need := count - len(picked)
	remain := append([]models.Word(nil), tested...)
	for need > 0 && len(remain) > 0 {
		idx := weightedIndex(remain, rng)
		picked = append(picked, remain[idx])
		remain = append(remain[:idx], remain[idx+1:]...)
		need--
	}
	return picked
}

func weightedIndex(words []models.Word, rng *rand.Rand) int {
	total := 0
	weights := make([]int, len(words))
	for i, w := range words {
		// Wrong answers weigh more so they reappear sooner.
		wgh := 1 + w.QuizWrong*3
		if wgh < 1 {
			wgh = 1
		}
		weights[i] = wgh
		total += wgh
	}
	if total <= 0 {
		return rng.Intn(len(words))
	}
	r := rng.Intn(total)
	for i, w := range weights {
		r -= w
		if r < 0 {
			return i
		}
	}
	return len(words) - 1
}

func buildOne(target models.Word, pool []models.Word, typ string, rng *rand.Rand) (models.QuizQuestion, bool) {
	term := strings.TrimSpace(target.Term)
	en := cleanMeaning(target.MeaningEN)
	zh := cleanMeaning(target.MeaningZH)

	switch typ {
	case models.QuizWordToDef, models.QuizClosestMeaning:
		if en == "" {
			return models.QuizQuestion{}, false
		}
		opts, idx, ok := mixOptions(en, collectEN(pool, target.ID), 4, rng)
		if !ok {
			return models.QuizQuestion{}, false
		}
		stem := fmt.Sprintf("Which definition best matches %q?", term)
		explain := fmt.Sprintf("%q means: %s", term, en)
		if typ == models.QuizClosestMeaning {
			stem = fmt.Sprintf("%q is closest in meaning to:", term)
			explain = fmt.Sprintf("The closest meaning of %q is: %s", term, en)
		}
		return models.QuizQuestion{
			ID: target.ID, Term: term, Type: typ,
			Question: stem, Options: opts, CorrectIndex: idx, ExplainAnswer: explain,
		}, true

	case models.QuizDefToWord:
		prompt := en
		if prompt == "" {
			prompt = zh
		}
		if prompt == "" {
			return models.QuizQuestion{}, false
		}
		opts, idx, ok := mixOptions(term, collectTerms(pool, target.ID), 4, rng)
		if !ok {
			return models.QuizQuestion{}, false
		}
		return models.QuizQuestion{
			ID: target.ID, Term: term, Type: models.QuizDefToWord,
			Question:      fmt.Sprintf("Which word or phrase best matches this meaning?\n%s", prompt),
			Options:       opts,
			CorrectIndex:  idx,
			ExplainAnswer: fmt.Sprintf("The answer is %q.", term),
		}, true

	case models.QuizZhToWord:
		if zh == "" {
			return models.QuizQuestion{}, false
		}
		opts, idx, ok := mixOptions(term, collectTerms(pool, target.ID), 4, rng)
		if !ok {
			return models.QuizQuestion{}, false
		}
		return models.QuizQuestion{
			ID: target.ID, Term: term, Type: models.QuizZhToWord,
			Question:      fmt.Sprintf("根据中文释义选出对应的英文单词/短语：\n%s", zh),
			Options:       opts,
			CorrectIndex:  idx,
			ExplainAnswer: fmt.Sprintf("答案是 %q。", term),
		}, true

	case models.QuizWordToZh:
		if zh == "" {
			return models.QuizQuestion{}, false
		}
		opts, idx, ok := mixOptions(zh, collectZH(pool, target.ID), 4, rng)
		if !ok {
			return models.QuizQuestion{}, false
		}
		return models.QuizQuestion{
			ID: target.ID, Term: term, Type: models.QuizWordToZh,
			Question:      fmt.Sprintf("Which Chinese meaning best matches %q?", term),
			Options:       opts,
			CorrectIndex:  idx,
			ExplainAnswer: fmt.Sprintf("%q ≈ %s", term, zh),
		}, true
	}
	return models.QuizQuestion{}, false
}

func cleanMeaning(s string) string {
	s = strings.TrimSpace(s)
	s = strings.ReplaceAll(s, "\r\n", "\n")
	return s
}

func collectEN(pool []models.Word, excludeID int64) []string {
	var out []string
	seen := map[string]bool{}
	for _, w := range pool {
		if w.ID == excludeID {
			continue
		}
		m := cleanMeaning(w.MeaningEN)
		if m == "" || seen[strings.ToLower(m)] {
			continue
		}
		seen[strings.ToLower(m)] = true
		out = append(out, m)
	}
	return out
}

func collectZH(pool []models.Word, excludeID int64) []string {
	var out []string
	seen := map[string]bool{}
	for _, w := range pool {
		if w.ID == excludeID {
			continue
		}
		m := cleanMeaning(w.MeaningZH)
		if m == "" || seen[m] {
			continue
		}
		seen[m] = true
		out = append(out, m)
	}
	return out
}

func collectTerms(pool []models.Word, excludeID int64) []string {
	var out []string
	seen := map[string]bool{}
	for _, w := range pool {
		if w.ID == excludeID {
			continue
		}
		t := strings.TrimSpace(w.Term)
		key := strings.ToLower(t)
		if t == "" || seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, t)
	}
	return out
}

func mixOptions(correct string, distractors []string, n int, rng *rand.Rand) ([]string, int, bool) {
	correct = strings.TrimSpace(correct)
	if correct == "" || n < 2 {
		return nil, 0, false
	}
	uniq := make([]string, 0, n)
	uniq = append(uniq, correct)
	seen := map[string]bool{strings.ToLower(correct): true}

	rng.Shuffle(len(distractors), func(i, j int) { distractors[i], distractors[j] = distractors[j], distractors[i] })
	for _, d := range distractors {
		d = strings.TrimSpace(d)
		key := strings.ToLower(d)
		if d == "" || seen[key] {
			continue
		}
		seen[key] = true
		uniq = append(uniq, d)
		if len(uniq) >= n {
			break
		}
	}
	if len(uniq) < 2 {
		return nil, 0, false
	}

	rng.Shuffle(len(uniq), func(i, j int) { uniq[i], uniq[j] = uniq[j], uniq[i] })
	idx := 0
	for i, o := range uniq {
		if strings.EqualFold(o, correct) || o == correct {
			idx = i
			break
		}
	}
	return uniq, idx, true
}
