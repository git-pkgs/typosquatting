package typosquatting_test

import (
	"errors"
	"reflect"
	"testing"

	typo "github.com/git-pkgs/typosquatting"
)

func TestCompareMatchesGeneration(t *testing.T) {
	tests := []struct {
		ecosystem typo.Ecosystem
		target    string
	}{
		{typo.Generic, "a-a-a"}, {typo.Generic, "café"},
		{typo.Generic, "modern"}, {typo.Generic, "foo-bar-baz"},
		{typo.Generic, "library2"}, {typo.Generic, "children"},
		{typo.PyPI, "My.__Package"}, {typo.Cargo, "My_Crate"},
		{typo.NPM, "@babel/core"}, {typo.RubyGems, "Atlas_Client"},
		{typo.GitHubActions, "Actions/Tool/Run@main"},
		{typo.Go, "github.com/shopspring/decimal/v2"},
		{typo.Go, "example.org/my-package/v2"},
	}
	for _, tt := range tests {
		t.Run(string(tt.ecosystem)+"/"+tt.target, func(t *testing.T) {
			options := typo.Options{Ecosystem: tt.ecosystem}
			variants, err := typo.Generate(tt.target, options)
			if err != nil {
				t.Fatal(err)
			}
			for _, variant := range variants {
				paths, compareErr := typo.Compare(variant.Name, tt.target, options)
				if compareErr != nil || !reflect.DeepEqual(paths, variant.Paths) {
					t.Fatalf("%s: got %v, %v; want %v", variant.Name, paths, compareErr, variant.Paths)
				}
			}
		})
	}
}

func TestCompareErrors(t *testing.T) {
	tests := []struct {
		candidate, target string
		options           typo.Options
		want              error
	}{
		{"", "requests", typo.Options{}, typo.ErrInvalidName},
		{"rrequests", "", typo.Options{}, typo.ErrInvalidName},
		{"rrequests", "requests", typo.Options{MaxCandidates: -1}, typo.ErrInvalidOptions},
		{"arequests", "requests", typo.Options{Algorithms: []typo.Algorithm{typo.Addition}, MaxCandidates: 1}, typo.ErrLimit},
		{"eight-seven-six-five-four-three-two-one", "one-two-three-four-five-six-seven-eight", typo.Options{Algorithms: []typo.Algorithm{typo.WordOrder}, MaxCandidates: 10}, typo.ErrLimit},
	}
	for _, tt := range tests {
		paths, err := typo.Compare(tt.candidate, tt.target, tt.options)
		if !errors.Is(err, tt.want) || paths != nil {
			t.Fatalf("%s: got %v, %v; want %v", tt.target, paths, err, tt.want)
		}
	}
	for _, options := range []typo.Options{{}, {Algorithms: []typo.Algorithm{}}} {
		paths, err := typo.Compare("unrelated", "requests", options)
		if err != nil || len(paths) != 0 {
			t.Fatalf("no match: %v, %v", paths, err)
		}
	}
}

func FuzzCompare(f *testing.F) {
	f.Add("rrequests", "requests")
	f.Add("foo-bar", "bar-foo")
	f.Add("cafe", "café")
	f.Fuzz(func(t *testing.T, candidate, target string) {
		if len(target) > 64 || len(candidate) > 64 {
			t.Skip()
		}
		if _, err := typo.Generic.Normalize(candidate); err != nil {
			t.Skip()
		}
		if candidate == target {
			return
		}
		options := typo.Options{MaxCandidates: 1000}
		variants, generateErr := typo.Generate(target, options)
		paths, compareErr := typo.Compare(candidate, target, options)
		if generateErr != nil {
			for _, sentinel := range []error{typo.ErrLimit, typo.ErrInvalidName} {
				if errors.Is(generateErr, sentinel) && !errors.Is(compareErr, sentinel) {
					t.Fatalf("generate: %v; compare: %v", generateErr, compareErr)
				}
			}
			return
		}
		if compareErr != nil {
			t.Fatal(compareErr)
		}
		var want []typo.Path
		for _, variant := range variants {
			if variant.Name == candidate {
				want = variant.Paths
			}
		}
		if !reflect.DeepEqual(paths, want) {
			t.Fatalf("got %v, want %v", paths, want)
		}
	})
}
