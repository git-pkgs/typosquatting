package typosquatting

import (
	"fmt"
	"slices"
	"sort"
)

// Package contains caller-supplied identity metadata. Owners should be stable
// registry account IDs; an empty set means ownership is unknown.
type Package struct {
	Name   string
	Owners []string
}

// Match identifies a corpus target and alternative paths to the candidate.
type Match struct {
	Candidate string
	Target    string
	Paths     []Path
}

type indexedMatch struct {
	target string
	paths  []Path
}

// Detector indexes variants of a caller-supplied corpus. It is safe for concurrent
// Check calls. Its memory use grows with the number of generated variants.
type Detector struct {
	ecosystem Ecosystem
	packages  map[string]Package
	variants  map[string][]indexedMatch
}

// NewDetector validates and copies the corpus, rejecting duplicate canonical
// names. Corpus selection and popularity thresholds are left to the caller.
func NewDetector(packages []Package, options Options) (*Detector, error) {
	options, err := options.validated()
	if err != nil {
		return nil, err
	}
	detector := &Detector{ecosystem: options.Ecosystem, packages: make(map[string]Package), variants: make(map[string][]indexedMatch)}
	for _, pkg := range packages {
		name, normalizeErr := options.Ecosystem.Normalize(pkg.Name)
		if normalizeErr != nil {
			return nil, normalizeErr
		}
		if _, exists := detector.packages[name]; exists {
			return nil, fmt.Errorf("duplicate corpus name: %q", name)
		}
		detector.packages[name] = Package{Name: name, Owners: slices.Clone(pkg.Owners)}
		variants, generateErr := Generate(name, options)
		if generateErr != nil {
			return nil, generateErr
		}
		for _, variant := range variants {
			detector.variants[variant.Name] = append(detector.variants[variant.Name], indexedMatch{target: name, paths: variant.Paths})
		}
	}
	return detector, nil
}

// Check returns matches sorted by target. Exact corpus members and pairs with
// shared owners are excluded. Returned slices do not alias the index.
func (d *Detector) Check(candidate Package) ([]Match, error) {
	name, err := d.ecosystem.Normalize(candidate.Name)
	if err != nil {
		return nil, err
	}
	if _, exists := d.packages[name]; exists {
		return nil, nil
	}
	var matches []Match
	for _, match := range d.variants[name] {
		if sharedOwner(candidate.Owners, d.packages[match.target].Owners) {
			continue
		}
		paths := make([]Path, len(match.paths))
		for i, path := range match.paths {
			paths[i] = slices.Clone(path)
		}
		matches = append(matches, Match{Candidate: name, Target: match.target, Paths: paths})
	}
	sort.Slice(matches, func(i, j int) bool { return matches[i].Target < matches[j].Target })
	return matches, nil
}

func sharedOwner(left, right []string) bool {
	for _, owner := range left {
		if owner != "" && slices.Contains(right, owner) {
			return true
		}
	}
	return false
}
