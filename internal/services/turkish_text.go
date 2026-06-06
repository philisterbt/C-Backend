// Package services uygulamanın iş mantığını (business logic) barındırır.
// Bu dosya AI çıktılarındaki bozuk Türkçe karakterleri ve yazım hatalarını düzeltir.
package services

import (
	"strings"
	"unicode"
)

// commonTypoFixes görü modelinin sık yaptığı bozuk Türkçe kalıpları ve doğruları.
// Deprem riski bağlamında en sık geçen kelimeler önceliklidir.
var commonTypoFixes = []struct{ wrong, right string }{
	{"güştştendirme", "güçlendirme"},
	{"güştşt", "güç"},
	{"güştendirme", "güçlendirme"},
	{"güşşşendirme", "güçlendirme"},
	{"güşendirme", "güçlendirme"},
	{"güşştendirme", "güçlendirme"},
	{"tahliyye", "tahliye"},
	{"tahlıye", "tahliye"},
	{"toplanmaa", "toplanma"},
	{"depremm", "deprem"},
	{"enkazz", "enkaz"},
	{"binaa", "bina"},
	{"sokaak", "sokak"},
	{"güzergaah", "güzergah"},
	{"güzergâh", "güzergah"},
	{"denetiim", "denetim"},
	{"denetimm", "denetim"},
	{"yapıı", "yapı"},
	{"yapi", "yapı"},
	{"oncereden", "önceden"},
	{"öncereden", "önceden"},
}

// NeedsLLMPolish metinde ciddi Türkçe bozulma (harf tekrarı, ş/ç karışıklığı) varsa true döner.
// Bu durumda ikinci bir metin modeli geçişi tetiklenir.
func NeedsLLMPolish(comment string, recommendations []string) bool {
	texts := append([]string{comment}, recommendations...)
	for _, t := range texts {
		if hasSuspiciousTurkish(t) {
			return true
		}
	}
	return false
}

// hasSuspiciousTurkish tek bir metinde şüpheli bozulma kalıplarını arar.
func hasSuspiciousTurkish(s string) bool {
	if hasTripleRepeatedRunes(s) {
		return true
	}
	lower := strings.ToLower(s)
	suspicious := []string{"güş", "güşt", "şş", "ççç", "ğğ", "üüü", "ööö", "ııı"}
	for _, sub := range suspicious {
		if strings.Contains(lower, sub) {
			return true
		}
	}
	return false
}

// hasTripleRepeatedRunes aynı harfin 3+ kez art arda tekrar edip etmediğini kontrol eder.
// Go regexp paketi \1 backreference desteklemediği için rune döngüsü kullanılır.
func hasTripleRepeatedRunes(s string) bool {
	runes := []rune(s)
	if len(runes) < 3 {
		return false
	}
	for i := 2; i < len(runes); i++ {
		if runes[i] == runes[i-1] && runes[i] == runes[i-2] {
			return true
		}
	}
	return false
}

// collapseTripleRepeatedRunes aynı harfin 3+ tekrarını tek harfe indirir (şşş → ş).
func collapseTripleRepeatedRunes(s string) string {
	runes := []rune(s)
	if len(runes) == 0 {
		return s
	}
	var b strings.Builder
	b.Grow(len(runes))

	i := 0
	for i < len(runes) {
		r := runes[i]
		j := i + 1
		for j < len(runes) && runes[j] == r {
			j++
		}
		count := j - i
		if count >= 3 {
			b.WriteRune(r) // 3+ tekrar → tek harf
		} else {
			for k := i; k < j; k++ {
				b.WriteRune(runes[k])
			}
		}
		i = j
	}
	return b.String()
}

// PolishTurkishTexts comment ve öneri listesindeki bozuk Türkçe metinleri düzeltir.
func PolishTurkishTexts(comment string, recommendations []string) (string, []string) {
	comment = polishTurkishString(comment)
	polished := make([]string, 0, len(recommendations))
	for _, rec := range recommendations {
		polished = append(polished, polishTurkishString(rec))
	}
	return comment, polished
}

// polishTurkishString tek bir metin üzerinde Türkçe düzeltme adımlarını uygular.
func polishTurkishString(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return s
	}

	// 1. Bilinen bozuk kalıpları doğru kelimelerle değiştir (case-insensitive)
	for _, fix := range commonTypoFixes {
		s = replaceIgnoreCase(s, fix.wrong, fix.right)
	}

	// 2. Aynı harfin 3+ tekrarını tek harfe indir (şşş → ş)
	s = collapseTripleRepeatedRunes(s)

	// 3. Harfler arası gereksiz boşlukları temizle
	s = collapseSpaces(s)

	// 4. Cümle başı büyük harf (Türkçe locale basit)
	if len(s) > 0 {
		runes := []rune(s)
		runes[0] = unicode.ToUpper(runes[0])
		s = string(runes)
	}

	return s
}

// replaceIgnoreCase büyük/küçük harf duyarsız metin değiştirme yapar.
func replaceIgnoreCase(s, old, new string) string {
	lower := strings.ToLower(s)
	oldLower := strings.ToLower(old)
	var result strings.Builder
	start := 0
	for {
		idx := strings.Index(lower[start:], oldLower)
		if idx == -1 {
			result.WriteString(s[start:])
			break
		}
		pos := start + idx
		result.WriteString(s[start:pos])
		result.WriteString(new)
		start = pos + len(old)
	}
	return result.String()
}

// collapseSpaces birden fazla ardışık boşluğu tek boşluğa indirir.
func collapseSpaces(s string) string {
	return strings.Join(strings.Fields(s), " ")
}
