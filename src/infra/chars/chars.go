package chars

import (
	"unicode"
)

func IsEnglishChar(r rune) bool {
	return ('a' <= r && r <= 'z') || ('A' <= r && r <= 'Z')
}

func IsJapaneseChar(r rune) bool {
	if 0x3040 <= r && r <= 0x309F {
		return true
	}
	if 0x30A0 <= r && r <= 0x30FF {
		return true
	}
	return false
}

func IsChineseChar(r rune) bool {
	if 0x4e00 <= r && r <= 0x9fff {
		if !(0x3040 <= r && r <= 0x309F || 0x30A0 <= r && r <= 0x30FF) {
			return true
		}
	}
	return false
}

func IsJapanesePunctuation(r rune) bool {
	return r == '\u30FC' || r == '\u301C' || r == '\u3002' || r == '\u3001'
}

func ratioOf(text string, predicate func(rune) bool) float64 {
	if len(text) == 0 {
		return 0.0
	}
	total := 0
	matched := 0
	for _, ch := range text {
		if unicode.IsSpace(ch) {
			continue
		}
		total++
		if predicate(ch) {
			matched++
		}
	}
	if total == 0 {
		return 0.0
	}
	return float64(matched) / float64(total)
}

func ContainsOriginalLanguage(text string) bool {
	if len(text) == 0 {
		return false
	}
	for _, c := range text {
		if IsJapaneseChar(c) || IsEnglishChar(c) {
			return true
		}
	}
	return false
}

func ContainsChinese(text string) bool {
	if len(text) == 0 {
		return false
	}
	for _, c := range text {
		if IsChineseChar(c) {
			return true
		}
	}
	return false
}

func IsPureChinese(text string) bool {
	return ratioOf(text, IsChineseChar) > 0.9
}

func IsMainlyJapanese(text string) bool {
	return ratioOf(text, func(ch rune) bool {
		return IsJapaneseChar(ch) || ch == '\u30FC' || ch == '\u301C' || ch == '\u3002' || ch == '\u3001'
	}) > 0.6
}

func IsMainlyOriginal(text string) bool {
	return ratioOf(text, func(ch rune) bool {
		return IsJapaneseChar(ch) || IsEnglishChar(ch) || ch == '\u30FC' || ch == '\u301C' || ch == '\u3002' || ch == '\u3001'
	}) > 0.5
}

func IsMainlyTranslation(text string) bool {
	return ratioOf(text, IsChineseChar) > 0.6
}

func CalcOriginalLanguageScore(text string) float64 {
	return ratioOf(text, func(ch rune) bool {
		return IsJapaneseChar(ch) || IsEnglishChar(ch)
	})
}

func CalcChinesePurityScore(text string) float64 {
	return ratioOf(text, IsChineseChar)
}

func CalcOriginalScore(text string) float64 {
	return ratioOf(text, func(ch rune) bool {
		return IsJapaneseChar(ch) || IsEnglishChar(ch) || ch == '\u30FC' || ch == '\u301C' || ch == '\u3002' || ch == '\u3001'
	})
}

func CalcTranslationScore(text string) float64 {
	return ratioOf(text, IsChineseChar)
}
