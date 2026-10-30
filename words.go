package typosquatting

import (
	"maps"
	"slices"
	"strings"
)

func commonMisspellings(name string, yield emit) bool {
	return wordReplacements(name, misspellings, false, yield)
}

func numeralWords(name string, yield emit) bool {
	return wordReplacements(name, numerals, true, yield)
}

func wordReplacements(name string, table map[string][]string, alternatives bool, yield emit) bool {
	for _, correct := range slices.Sorted(maps.Keys(table)) {
		words := table[correct]
		if strings.Contains(name, correct) {
			for _, word := range words {
				if !yield(strings.ReplaceAll(name, correct, word)) {
					return false
				}
			}
		}
		for _, word := range words {
			if !strings.Contains(name, word) {
				continue
			}
			if !yield(strings.ReplaceAll(name, word, correct)) {
				return false
			}
			if alternatives && !replaceAlternatives(name, word, words, yield) {
				return false
			}
		}
	}
	return true
}

func replaceAlternatives(name, word string, alternatives []string, yield emit) bool {
	for _, other := range alternatives {
		if other != word && !yield(strings.ReplaceAll(name, word, other)) {
			return false
		}
	}
	return true
}

func plurals(name string, yield emit) bool {
	return yield(pluralize(name)) && yield(singularize(name))
}

func pluralize(word string) string {
	if plural, ok := irregularPlurals[word]; ok {
		return plural
	}
	switch {
	case strings.HasSuffix(word, "y") && len(word) > 1 && !strings.ContainsRune("aeiou", rune(word[len(word)-2])):
		return strings.TrimSuffix(word, "y") + "ies"
	case endsWithAny(word, "ss", "x", "z", "ch", "sh", "s"):
		return word + "es"
	case strings.HasSuffix(word, "fe"):
		return strings.TrimSuffix(word, "fe") + "ves"
	case strings.HasSuffix(word, "f"):
		return strings.TrimSuffix(word, "f") + "ves"
	default:
		return word + "s"
	}
}

func singularize(word string) string {
	for _, singular := range slices.Sorted(maps.Keys(irregularPlurals)) {
		if irregularPlurals[singular] == word {
			return singular
		}
	}
	switch {
	case strings.HasSuffix(word, "ies"):
		return strings.TrimSuffix(word, "ies") + "y"
	case strings.HasSuffix(word, "ves"):
		return strings.TrimSuffix(word, "ves") + "f"
	case endsWithAny(word, "ses", "xes", "zes", "ches", "shes"):
		return strings.TrimSuffix(word, "es")
	default:
		return strings.TrimSuffix(word, "s")
	}
}

func endsWithAny(word string, suffixes ...string) bool {
	for _, suffix := range suffixes {
		if strings.HasSuffix(word, suffix) {
			return true
		}
	}
	return false
}
