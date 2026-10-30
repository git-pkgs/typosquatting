package typosquatting

import (
	"maps"
	"slices"
	"strings"
	"unicode"
)

type emit func(string) bool
type generator func(string, emit) bool

var generators = map[Algorithm]generator{
	Omission: omissions, Repetition: repetitions, Replacement: replacements,
	Transposition: transpositions, Addition: additions, Homoglyph: homoglyphs,
	VowelSwap: vowelSwaps, Delimiter: delimiters, WordOrder: wordOrders,
	Plural: plurals, Misspelling: commonMisspellings, Numeral: numeralWords,
	Bitflip: bitflips, AdjacentInsertion: adjacentInsertions,
	DoubleHit: doubleHits, Combosquatting: combosquats,
}

const (
	lettersAndDigits = "abcdefghijklmnopqrstuvwxyz0123456789"
	delimiterChars   = "-_."
	vowels           = "aeiouy"
	asciiBits        = 7
	pairWidth        = 2
)

func splice(chars []rune, start, end int, replacement string) string {
	return string(chars[:start]) + replacement + string(chars[end:])
}

func omissions(name string, yield emit) bool {
	chars := []rune(name)
	for i := range chars {
		if !yield(splice(chars, i, i+1, "")) {
			return false
		}
	}
	return true
}

func repetitions(name string, yield emit) bool {
	chars := []rune(name)
	for i, char := range chars {
		if !yield(splice(chars, i, i, string(char))) {
			return false
		}
	}
	return true
}

func replacements(name string, yield emit) bool {
	chars := []rune(name)
	for i, char := range chars {
		for _, adjacent := range keyboard[unicode.ToLower(char)] {
			if unicode.IsUpper(char) {
				adjacent = unicode.ToUpper(adjacent)
			}
			if !yield(splice(chars, i, i+1, string(adjacent))) {
				return false
			}
		}
	}
	return true
}

func transpositions(name string, yield emit) bool {
	chars := []rune(name)
	for i := 0; i+1 < len(chars); i++ {
		if !yield(splice(chars, i, i+pairWidth, string([]rune{chars[i+1], chars[i]}))) {
			return false
		}
	}
	return true
}

func additions(name string, yield emit) bool {
	for _, char := range lettersAndDigits {
		if !yield(string(char)+name) || !yield(name+string(char)) {
			return false
		}
	}
	return true
}

func homoglyphs(name string, yield emit) bool {
	chars := []rune(name)
	for i, char := range chars {
		for _, glyph := range glyphs[string(unicode.ToLower(char))] {
			if !yield(splice(chars, i, i+1, glyph)) {
				return false
			}
		}
	}
	for _, pattern := range slices.Sorted(maps.Keys(glyphs)) {
		if len(pattern) == 1 || !strings.Contains(name, pattern) {
			continue
		}
		for _, replacement := range glyphs[pattern] {
			if !yield(strings.ReplaceAll(name, pattern, replacement)) {
				return false
			}
		}
	}
	return true
}

func vowelSwaps(name string, yield emit) bool {
	chars := []rune(name)
	for i, char := range chars {
		if !strings.ContainsRune(vowels, unicode.ToLower(char)) {
			continue
		}
		for _, vowel := range vowels {
			if unicode.IsUpper(char) {
				vowel = unicode.ToUpper(vowel)
			}
			if !yield(splice(chars, i, i+1, string(vowel))) {
				return false
			}
		}
	}
	return true
}

func delimiters(name string, yield emit) bool {
	for _, from := range delimiterChars {
		if !strings.ContainsRune(name, from) {
			continue
		}
		for _, to := range []string{"-", "_", ".", ""} {
			if to == string(from) {
				continue
			}
			current := name
			for strings.ContainsRune(current, from) {
				current = strings.Replace(current, string(from), to, 1)
				if !yield(current) {
					return false
				}
			}
		}
	}
	chars := []rune(name)
	for i := 1; i < len(chars); i++ {
		if strings.ContainsRune(delimiterChars, chars[i-1]) || strings.ContainsRune(delimiterChars, chars[i]) {
			continue
		}
		for _, delimiter := range delimiterChars {
			if !yield(splice(chars, i, i, string(delimiter))) {
				return false
			}
		}
	}
	return true
}

func wordOrders(name string, yield emit) bool {
	for _, delimiter := range delimiterChars {
		parts := strings.Split(name, string(delimiter))
		if len(parts) < pairWidth {
			continue
		}
		if !permutations(parts, string(delimiter), yield) {
			return false
		}
	}
	return true
}

func permutations(parts []string, delimiter string, yield emit) bool {
	slices.Sort(parts)
	for {
		if !yield(strings.Join(parts, delimiter)) {
			return false
		}
		if !nextPermutation(parts) {
			return true
		}
	}
}

func nextPermutation(parts []string) bool {
	pivot := len(parts) - pairWidth
	for pivot >= 0 && parts[pivot] >= parts[pivot+1] {
		pivot--
	}
	if pivot < 0 {
		return false
	}
	next := len(parts) - 1
	for parts[next] <= parts[pivot] {
		next--
	}
	parts[pivot], parts[next] = parts[next], parts[pivot]
	slices.Reverse(parts[pivot+1:])
	return true
}

func bitflips(name string, yield emit) bool {
	chars := []rune(name)
	for i, char := range chars {
		if char > unicode.MaxASCII {
			continue
		}
		for bit := range asciiBits {
			flipped := char ^ (1 << bit)
			if strings.ContainsRune(lettersAndDigits+"-_", flipped) && !yield(splice(chars, i, i+1, string(flipped))) {
				return false
			}
		}
	}
	return true
}

func adjacentInsertions(name string, yield emit) bool {
	chars := []rune(name)
	for i, char := range chars {
		for _, adjacent := range keyboard[unicode.ToLower(char)] {
			if !yield(splice(chars, i, i, string(adjacent))) || !yield(splice(chars, i+1, i+1, string(adjacent))) {
				return false
			}
		}
	}
	return true
}

func doubleHits(name string, yield emit) bool {
	chars := []rune(name)
	for i := 0; i+1 < len(chars); i++ {
		if chars[i] != chars[i+1] {
			continue
		}
		for _, adjacent := range keyboard[unicode.ToLower(chars[i])] {
			if !yield(splice(chars, i, i+pairWidth, strings.Repeat(string(adjacent), pairWidth))) {
				return false
			}
		}
	}
	return true
}

func combosquats(name string, yield emit) bool {
	for _, prefix := range comboPrefixes {
		if !yield(prefix + name) {
			return false
		}
	}
	for _, suffix := range comboSuffixes {
		if !yield(name + suffix) {
			return false
		}
	}
	return true
}
