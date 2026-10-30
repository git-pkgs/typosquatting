package typosquatting

import (
	"context"
	"errors"
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
	Candidate    string
	Target       string
	Paths        []Path
	CorpusMember bool
	SharedOwner  bool
}

// CheckOptions controls exclusions. The zero value excludes corpus members and
// pairs with shared owners. Including either retains its flag on each Match.
type CheckOptions struct {
	IncludeCorpusMembers bool
	IncludeSharedOwners  bool
}

var (
	ErrDuplicateName = errors.New("duplicate corpus name")
	ErrIndexLimit    = errors.New("detector index limit exceeded")
)

// BuildOptions controls corpus indexing. MaxEntries bounds distinct candidate-target
// pairs, including pairs excluded by Check. Zero is unlimited; this is not a byte limit.
// SkipDuplicates keeps the first package's owners. Skipped entries are reported in Stats.
type BuildOptions struct {
	MaxEntries     int
	SkipInvalid    bool
	SkipDuplicates bool
}

// CorpusIssue identifies a skipped input by its zero-based index and original name.
type CorpusIssue struct {
	Index int
	Name  string
	Err   error
}

// BuildStats counts indexed packages, distinct candidate names, and candidate-target
// pairs. Skipped entries are ordered by input position.
type BuildStats struct {
	Packages int
	Variants int
	Entries  int
	Skipped  []CorpusIssue
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
	stats     BuildStats
}

// NewDetector validates and copies the corpus, rejecting duplicate canonical
// names. Corpus selection and popularity thresholds are left to the caller.
func NewDetector(packages []Package, options Options) (*Detector, error) {
	return NewDetectorContext(context.Background(), packages, options, BuildOptions{})
}

// NewDetectorContext builds a cancellable index with optional limits and corpus
// validation skips. Errors return no partial detector. Generation limits and
// cancellation always fail the build, even when skips are enabled.
func NewDetectorContext(ctx context.Context, packages []Package, options Options, build BuildOptions) (*Detector, error) {
	options, err := options.validated()
	if err != nil {
		return nil, err
	}
	if build.MaxEntries < 0 {
		return nil, ErrInvalidOptions
	}
	detector := &Detector{ecosystem: options.Ecosystem, packages: make(map[string]Package), variants: make(map[string][]indexedMatch)}
	for i, pkg := range packages {
		if err = ctx.Err(); err != nil {
			return nil, err
		}
		if err = detector.addPackage(ctx, pkg, options, build.MaxEntries); err != nil {
			if (build.SkipInvalid && errors.Is(err, ErrInvalidName)) || (build.SkipDuplicates && errors.Is(err, ErrDuplicateName)) {
				detector.stats.Skipped = append(detector.stats.Skipped, CorpusIssue{Index: i, Name: pkg.Name, Err: err})
				continue
			}
			return nil, fmt.Errorf("corpus entry %d (%q): %w", i, pkg.Name, err)
		}
	}
	if err = ctx.Err(); err != nil {
		return nil, err
	}
	return detector, nil
}

func (d *Detector) addPackage(ctx context.Context, pkg Package, options Options, maxEntries int) error {
	name, err := d.ecosystem.Normalize(pkg.Name)
	if err != nil {
		return err
	}
	if _, exists := d.packages[name]; exists {
		return fmt.Errorf("%w: %q", ErrDuplicateName, name)
	}
	err = walkVariants(ctx, name, options, func(candidate string, algorithm Algorithm) error {
		matches := d.variants[candidate]
		// All paths for a target are indexed together.
		if len(matches) > 0 && matches[len(matches)-1].target == name {
			match := &matches[len(matches)-1]
			match.paths = appendPath(match.paths, algorithm)
			return nil
		}
		if maxEntries > 0 && d.stats.Entries >= maxEntries {
			return ErrIndexLimit
		}
		d.variants[candidate] = append(matches, indexedMatch{target: name, paths: []Path{{algorithm}}})
		d.stats.Entries++
		return nil
	})
	if err != nil {
		return err
	}
	d.packages[name] = Package{Name: name, Owners: slices.Clone(pkg.Owners)}
	d.stats.Packages++
	d.stats.Variants = len(d.variants)
	return nil
}

// Stats returns build counts and a copy of the skipped-entry diagnostics.
func (d *Detector) Stats() BuildStats {
	stats := d.stats
	stats.Skipped = slices.Clone(stats.Skipped)
	return stats
}

// Check returns matches sorted by target. Exact corpus members and pairs with
// shared owners are excluded. Returned slices do not alias the index.
func (d *Detector) Check(candidate Package) ([]Match, error) {
	return d.CheckWithOptions(candidate, CheckOptions{})
}

// CheckWithOptions returns matches sorted by target, with caller-selected exclusions.
// CorpusMember describes the candidate; SharedOwner describes each pair using the
// candidate's supplied owner IDs. Equal candidate and target names never match.
func (d *Detector) CheckWithOptions(candidate Package, options CheckOptions) ([]Match, error) {
	name, err := d.ecosystem.Normalize(candidate.Name)
	if err != nil {
		return nil, err
	}
	_, corpusMember := d.packages[name]
	if corpusMember && !options.IncludeCorpusMembers {
		return nil, nil
	}
	var matches []Match
	for _, match := range d.variants[name] {
		shared := sharedOwner(candidate.Owners, d.packages[match.target].Owners)
		if shared && !options.IncludeSharedOwners {
			continue
		}
		paths := make([]Path, len(match.paths))
		for i, path := range match.paths {
			paths[i] = slices.Clone(path)
		}
		matches = append(matches, Match{Candidate: name, Target: match.target, Paths: paths, CorpusMember: corpusMember, SharedOwner: shared})
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
