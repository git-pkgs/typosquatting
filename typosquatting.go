// Package typosquatting generates and detects similar package names without network access.
package typosquatting

import (
	"cmp"
	"errors"
	"fmt"
	"slices"
	"sort"
)

// Algorithm identifies a spelling transformation from a target to a candidate.
type Algorithm string

const (
	Omission          Algorithm = "omission"
	Repetition        Algorithm = "repetition"
	Replacement       Algorithm = "replacement"
	Transposition     Algorithm = "transposition"
	Addition          Algorithm = "addition"
	Homoglyph         Algorithm = "homoglyph"
	VowelSwap         Algorithm = "vowel_swap"
	Delimiter         Algorithm = "delimiter"
	WordOrder         Algorithm = "word_order"
	Plural            Algorithm = "plural"
	Misspelling       Algorithm = "misspelling"
	Numeral           Algorithm = "numeral"
	Bitflip           Algorithm = "bitflip"
	AdjacentInsertion Algorithm = "adjacent_insertion"
	DoubleHit         Algorithm = "double_hit"
	Combosquatting    Algorithm = "combosquatting"
)

var (
	ErrInvalidName    = errors.New("invalid package name")
	ErrInvalidOptions = errors.New("invalid options")
	ErrLimit          = errors.New("candidate generation limit exceeded")
)

const defaultMaxCandidates = 10000

// Options configures generation and matching. Nil Algorithms selects all algorithms;
// an empty slice selects none. MaxCandidates bounds raw candidates, including duplicates
// and invalid names, per target. Zero uses 10,000.
type Options struct {
	Ecosystem     Ecosystem
	Algorithms    []Algorithm
	MaxCandidates int
}

// Path is an ordered sequence of transformations from target to candidate.
// Each generated path currently contains one algorithm.
type Path []Algorithm

// Variant holds a canonical name and alternative transformation paths.
type Variant struct {
	Name  string
	Paths []Path
}

// Algorithms returns the supported algorithms in stable order.
func Algorithms() []Algorithm {
	return []Algorithm{Omission, Repetition, Replacement, Transposition, Addition,
		Homoglyph, VowelSwap, Delimiter, WordOrder, Plural, Misspelling, Numeral,
		Bitflip, AdjacentInsertion, DoubleHit, Combosquatting}
}

func (o Options) validated() (Options, error) {
	if !o.Ecosystem.supported() || o.MaxCandidates < 0 {
		return o, ErrInvalidOptions
	}
	if o.MaxCandidates == 0 {
		o.MaxCandidates = defaultMaxCandidates
	}
	if o.Algorithms == nil {
		o.Algorithms = Algorithms()
	}
	o.Algorithms = slices.Clone(o.Algorithms)
	slices.Sort(o.Algorithms)
	o.Algorithms = slices.Compact(o.Algorithms)
	for _, algorithm := range o.Algorithms {
		if _, ok := generators[algorithm]; !ok {
			return o, fmt.Errorf("%w: algorithm %q", ErrInvalidOptions, algorithm)
		}
	}
	return o, nil
}

// Generate returns distinct canonical variants sorted by name. It returns no
// partial results when the generation limit is exceeded.
func Generate(target string, options Options) ([]Variant, error) {
	options, err := options.validated()
	if err != nil {
		return nil, err
	}
	name, err := options.Ecosystem.Normalize(target)
	if err != nil {
		return nil, err
	}
	results := make(map[string][]Path)
	attempted := 0
	for _, part := range options.Ecosystem.parts(name) {
		for _, algorithm := range options.Algorithms {
			complete := generators[algorithm](part.text, func(candidate string) bool {
				attempted++
				if attempted > options.MaxCandidates {
					return false
				}
				canonical, normalizeErr := options.Ecosystem.Normalize(part.prefix + candidate + part.suffix)
				if normalizeErr != nil || canonical == name {
					return true
				}
				path := Path{algorithm}
				if !slices.ContainsFunc(results[canonical], func(existing Path) bool { return slices.Equal(existing, path) }) {
					results[canonical] = append(results[canonical], path)
				}
				return true
			})
			if !complete {
				return nil, fmt.Errorf("%w for %q", ErrLimit, target)
			}
		}
	}
	variants := make([]Variant, 0, len(results))
	for candidate, paths := range results {
		variants = append(variants, Variant{Name: candidate, Paths: paths})
	}
	sort.Slice(variants, func(i, j int) bool { return variants[i].Name < variants[j].Name })
	return variants, nil
}

// Compare returns paths from target to candidate, after ecosystem normalization.
// Equivalent names return no paths.
func Compare(candidate, target string, options Options) ([]Path, error) {
	options, err := options.validated()
	if err != nil {
		return nil, err
	}
	canonical, err := options.Ecosystem.Normalize(candidate)
	if err != nil {
		return nil, err
	}
	targetName, err := options.Ecosystem.Normalize(target)
	if err != nil {
		return nil, err
	}
	if canonical == targetName {
		return nil, nil
	}
	variants, err := Generate(target, options)
	if err != nil {
		return nil, err
	}
	index, found := slices.BinarySearchFunc(variants, canonical, func(v Variant, name string) int {
		return cmp.Compare(v.Name, name)
	})
	if !found {
		return nil, nil
	}
	return variants[index].Paths, nil
}
