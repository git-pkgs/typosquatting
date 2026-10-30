package typosquatting_test

import (
	"context"
	"errors"
	"reflect"
	"sync"
	"testing"

	typo "github.com/git-pkgs/typosquatting"
)

func TestCheckOptions(t *testing.T) {
	detector, err := typo.NewDetector([]typo.Package{
		{Name: "requests", Owners: []string{"alice"}},
		{Name: "rrequests"},
		{Name: "rrrequests", Owners: []string{"bob"}},
	}, typo.Options{Ecosystem: typo.PyPI, Algorithms: []typo.Algorithm{typo.Omission, typo.Repetition}})
	if err != nil {
		t.Fatal(err)
	}
	candidate := typo.Package{Name: "RRequests", Owners: []string{"alice"}}
	tests := []struct {
		name    string
		options typo.CheckOptions
		targets []string
	}{
		{"defaults", typo.CheckOptions{}, nil},
		{"corpus members", typo.CheckOptions{IncludeCorpusMembers: true}, []string{"rrrequests"}},
		{"shared owners", typo.CheckOptions{IncludeSharedOwners: true}, nil},
		{"all", typo.CheckOptions{IncludeCorpusMembers: true, IncludeSharedOwners: true}, []string{"requests", "rrrequests"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			matches, checkErr := detector.CheckWithOptions(candidate, tt.options)
			if checkErr != nil {
				t.Fatal(checkErr)
			}
			var targets []string
			for _, match := range matches {
				targets = append(targets, match.Target)
				if match.Candidate != "rrequests" || !match.CorpusMember || match.SharedOwner != (match.Target == "requests") {
					t.Fatalf("wrong explanation: %+v", match)
				}
			}
			if !reflect.DeepEqual(targets, tt.targets) {
				t.Fatalf("targets %v, want %v", targets, tt.targets)
			}
		})
	}
	outsider := typo.Package{Name: "rrrrequests", Owners: []string{"bob"}}
	if matches, checkErr := detector.Check(outsider); checkErr != nil || len(matches) != 0 {
		t.Fatalf("shared-owner default: %v, %v", matches, checkErr)
	}
	matches, err := detector.CheckWithOptions(outsider, typo.CheckOptions{IncludeSharedOwners: true})
	if err != nil || len(matches) != 1 || matches[0].CorpusMember || !matches[0].SharedOwner {
		t.Fatalf("shared-owner opt-in: %v, %v", matches, err)
	}
	matches[0].Paths[0][0] = "changed"
	again, err := detector.CheckWithOptions(outsider, typo.CheckOptions{IncludeSharedOwners: true})
	if err != nil || again[0].Paths[0][0] == "changed" {
		t.Fatal("result aliases index")
	}
}

func TestCheckUnknownOwners(t *testing.T) {
	detector, err := typo.NewDetector([]typo.Package{{Name: "requests", Owners: []string{""}}}, typo.Options{})
	if err != nil {
		t.Fatal(err)
	}
	matches, err := detector.CheckWithOptions(typo.Package{Name: "rrequests", Owners: []string{""}}, typo.CheckOptions{})
	if err != nil || len(matches) != 1 || matches[0].SharedOwner || matches[0].CorpusMember {
		t.Fatalf("unknown owners: %v, %v", matches, err)
	}
	if matches, err = detector.CheckWithOptions(typo.Package{Name: ""}, typo.CheckOptions{}); !errors.Is(err, typo.ErrInvalidName) || matches != nil {
		t.Fatalf("invalid candidate: %v, %v", matches, err)
	}
}

func TestDetectorBuildLimit(t *testing.T) {
	packages := []typo.Package{{Name: "aaa"}, {Name: "a"}}
	options := typo.Options{Algorithms: []typo.Algorithm{typo.Omission, typo.Repetition}}
	for _, limit := range []int{0, 3} {
		detector, err := typo.NewDetectorContext(context.Background(), packages, options, typo.BuildOptions{MaxEntries: limit})
		if err != nil {
			t.Fatal(err)
		}
		want := typo.BuildStats{Packages: 2, Variants: 2, Entries: 3}
		if stats := detector.Stats(); !reflect.DeepEqual(stats, want) {
			t.Fatalf("stats %+v, want %+v", stats, want)
		}
		matches, checkErr := detector.Check(typo.Package{Name: "aa"})
		if checkErr != nil || len(matches) != 2 || matches[0].Target != "a" || matches[1].Target != "aaa" {
			t.Fatalf("shared candidate: %v, %v", matches, checkErr)
		}
	}
	for _, limit := range []int{1, 2} {
		detector, err := typo.NewDetectorContext(context.Background(), packages, options, typo.BuildOptions{
			MaxEntries: limit, SkipInvalid: true, SkipDuplicates: true,
		})
		if !errors.Is(err, typo.ErrIndexLimit) || detector != nil {
			t.Fatalf("limit %d: %v, %v", limit, detector, err)
		}
	}
}

func TestDetectorBuildRetainsPaths(t *testing.T) {
	options := typo.Options{Ecosystem: typo.PyPI}
	variants, err := typo.Generate("requests", options)
	if err != nil {
		t.Fatal(err)
	}
	detector, err := typo.NewDetectorContext(context.Background(), []typo.Package{{Name: "requests"}}, options, typo.BuildOptions{
		MaxEntries: len(variants),
	})
	if err != nil {
		t.Fatal(err)
	}
	if stats := detector.Stats(); stats.Entries != len(variants) || stats.Variants != len(variants) {
		t.Fatalf("multiple paths counted as separate entries: %+v", stats)
	}
	for _, variant := range variants {
		matches, checkErr := detector.Check(typo.Package{Name: variant.Name})
		if checkErr != nil || len(matches) != 1 || !reflect.DeepEqual(matches[0].Paths, variant.Paths) {
			t.Fatalf("%s: got %v, %v; want %v", variant.Name, matches, checkErr, variant.Paths)
		}
	}
}

func TestDetectorBuildSkips(t *testing.T) {
	owners := []string{"alice"}
	packages := []typo.Package{
		{Name: "Requests", Owners: owners},
		{Name: "bad/name"},
		{Name: "requests", Owners: []string{"bob"}},
		{Name: "flask"},
	}
	detector, err := typo.NewDetectorContext(context.Background(), packages, typo.Options{Ecosystem: typo.PyPI}, typo.BuildOptions{
		SkipInvalid: true, SkipDuplicates: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	owners[0] = "bob"
	packages[0].Name = "changed"
	stats := detector.Stats()
	if stats.Packages != 2 || stats.Entries < stats.Variants || len(stats.Skipped) != 2 {
		t.Fatalf("stats: %+v", stats)
	}
	for i, wantErr := range []error{typo.ErrInvalidName, typo.ErrDuplicateName} {
		issue := stats.Skipped[i]
		if issue.Index != i+1 || issue.Name != packages[i+1].Name || !errors.Is(issue.Err, wantErr) {
			t.Fatalf("issue: %+v", issue)
		}
	}
	stats.Skipped[0].Name = "changed"
	if detector.Stats().Skipped[0].Name != "bad/name" {
		t.Fatal("stats aliases diagnostics")
	}
	for _, owner := range []string{"alice", "bob"} {
		matches, checkErr := detector.Check(typo.Package{Name: "rrequests", Owners: []string{owner}})
		if checkErr != nil || (len(matches) != 0) != (owner == "bob") {
			t.Fatalf("owner %s: %v, %v", owner, matches, checkErr)
		}
	}
	var group sync.WaitGroup
	for range 8 {
		group.Go(func() {
			got := detector.Stats()
			got.Skipped[0].Name = "changed"
			if _, checkErr := detector.CheckWithOptions(typo.Package{Name: "rrequests"}, typo.CheckOptions{IncludeSharedOwners: true}); checkErr != nil {
				t.Error(checkErr)
			}
		})
	}
	group.Wait()
}

func TestDetectorBuildErrors(t *testing.T) {
	tests := []struct {
		name     string
		packages []typo.Package
		options  typo.Options
		build    typo.BuildOptions
		want     error
	}{
		{"negative limit", nil, typo.Options{}, typo.BuildOptions{MaxEntries: -1}, typo.ErrInvalidOptions},
		{"invalid options", nil, typo.Options{Ecosystem: "unknown"}, typo.BuildOptions{}, typo.ErrInvalidOptions},
		{"invalid name", []typo.Package{{Name: ""}}, typo.Options{}, typo.BuildOptions{SkipDuplicates: true}, typo.ErrInvalidName},
		{"duplicate name", []typo.Package{{Name: "a"}, {Name: "A"}}, typo.Options{Ecosystem: typo.PyPI}, typo.BuildOptions{SkipInvalid: true}, typo.ErrDuplicateName},
		{"generation limit", []typo.Package{{Name: "requests"}}, typo.Options{MaxCandidates: 1}, typo.BuildOptions{SkipInvalid: true, SkipDuplicates: true}, typo.ErrLimit},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			detector, err := typo.NewDetectorContext(context.Background(), tt.packages, tt.options, tt.build)
			if !errors.Is(err, tt.want) || detector != nil {
				t.Fatalf("got %v, %v; want %v", detector, err, tt.want)
			}
		})
	}
}

func TestDetectorBuildEmpty(t *testing.T) {
	for _, packages := range [][]typo.Package{nil, {{Name: ""}}} {
		detector, err := typo.NewDetectorContext(context.Background(), packages, typo.Options{}, typo.BuildOptions{SkipInvalid: true})
		if err != nil {
			t.Fatal(err)
		}
		if stats := detector.Stats(); stats.Packages != 0 || stats.Entries != 0 || stats.Variants != 0 || len(stats.Skipped) != len(packages) {
			t.Fatalf("empty stats: %+v", stats)
		}
		matches, checkErr := detector.Check(typo.Package{Name: "requests"})
		if checkErr != nil || len(matches) != 0 {
			t.Fatalf("empty index: %v, %v", matches, checkErr)
		}
	}
}

func TestDetectorBuildCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	for _, packages := range [][]typo.Package{nil, {{Name: "requests"}}} {
		detector, err := typo.NewDetectorContext(ctx, packages, typo.Options{}, typo.BuildOptions{})
		if !errors.Is(err, context.Canceled) || detector != nil {
			t.Fatalf("canceled build: %v, %v", detector, err)
		}
	}
	ctx, cancel = context.WithCancel(context.Background())
	defer cancel()
	midBuild := &cancelDuringBuild{Context: ctx, cancel: cancel, remaining: 20}
	detector, err := typo.NewDetectorContext(midBuild, []typo.Package{{Name: "requests"}}, typo.Options{}, typo.BuildOptions{
		SkipInvalid: true, SkipDuplicates: true,
	})
	if !errors.Is(err, context.Canceled) || detector != nil || midBuild.remaining != 0 {
		t.Fatalf("mid-build cancellation: %v, %v", detector, err)
	}
}

// Cancel after several context checks to exercise cancellation within one target.
type cancelDuringBuild struct {
	context.Context
	cancel    context.CancelFunc
	remaining int
}

func (c *cancelDuringBuild) Err() error {
	c.remaining--
	if c.remaining == 0 {
		c.cancel()
	}
	return c.Context.Err()
}
