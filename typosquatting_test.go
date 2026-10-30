package typosquatting_test

import (
	"errors"
	"fmt"
	"reflect"
	"slices"
	"strings"
	"sync"
	"testing"

	typo "github.com/git-pkgs/typosquatting"
)

func TestAlgorithms(t *testing.T) {
	tests := []struct {
		algorithm         typo.Algorithm
		target, candidate string
	}{
		{typo.Omission, "requests", "reqests"},
		{typo.Repetition, "requests", "rrequests"},
		{typo.Replacement, "numpy", "numoy"},
		{typo.Transposition, "lodash", "loadsh"},
		{typo.Addition, "requests", "arequests"},
		{typo.Homoglyph, "modern", "modem"},
		{typo.VowelSwap, "requests", "raquests"},
		{typo.Delimiter, "atlas_client", "atlas-client"},
		{typo.WordOrder, "foo-bar-baz", "baz-foo-bar"},
		{typo.Plural, "child", "children"},
		{typo.Misspelling, "library", "libary"},
		{typo.Numeral, "lib2", "libtwo"},
		{typo.Bitflip, "google", "coogle"},
		{typo.AdjacentInsertion, "google", "googhle"},
		{typo.DoubleHit, "google", "giigle"},
		{typo.Combosquatting, "lodash", "lodash-js"},
	}
	for _, tt := range tests {
		t.Run(string(tt.algorithm), func(t *testing.T) {
			paths, err := typo.Compare(tt.candidate, tt.target, typo.Options{Algorithms: []typo.Algorithm{tt.algorithm}})
			if err != nil {
				t.Fatal(err)
			}
			want := []typo.Path{{tt.algorithm}}
			if !reflect.DeepEqual(paths, want) {
				t.Fatalf("got %v, want %v", paths, want)
			}
			_, err = typo.Generate(tt.target, typo.Options{Algorithms: []typo.Algorithm{tt.algorithm}, MaxCandidates: 1})
			if !errors.Is(err, typo.ErrLimit) {
				t.Fatalf("generation budget: %v", err)
			}
		})
	}
	if len(tests) != len(typo.Algorithms()) {
		t.Fatal("missing algorithm case")
	}
}

func TestAlternativePaths(t *testing.T) {
	paths, err := typo.Compare("rrequests", "requests", typo.Options{})
	if err != nil {
		t.Fatal(err)
	}
	want := []typo.Path{{typo.Addition}, {typo.AdjacentInsertion}, {typo.Repetition}}
	if !reflect.DeepEqual(paths, want) {
		t.Fatalf("got %v, want %v", paths, want)
	}
	for _, path := range paths {
		if len(path) != 1 {
			t.Fatal("alternatives were combined into a sequence")
		}
	}
}

func TestEcosystems(t *testing.T) {
	tests := []struct {
		name              string
		ecosystem         typo.Ecosystem
		candidate, target string
		match             bool
	}{
		{"pypi typo", typo.PyPI, "Reqeusts", "Requests", true},
		{"pypi equivalents", typo.PyPI, "MY.__PACKAGE", "my-package", false},
		{"cargo equivalents", typo.Cargo, "RUST_DECIMAL", "rust-decimal", false},
		{"cargo omission", typo.Cargo, "rustdecimal", "rust_decimal", true},
		{"gems delimiters", typo.RubyGems, "atlas-client", "atlas_client", true},
		{"npm scope", typo.NPM, "@babelx/core", "@babel/core", true},
		{"npm same scope", typo.NPM, "@babel/cre", "@babel/core", false},
		{"action owner", typo.GitHubActions, "ACTI0NS/Checkout@main", "actions/checkout@v4", true},
		{"action same owner", typo.GitHubActions, "actions/chckout", "actions/checkout", false},
		{"action refs", typo.GitHubActions, "actions/checkout@main", "Actions/Checkout@v4", false},
		{"action subpath", typo.GitHubActions, "actons/tool/Run@main", "actions/tool/Run@v4", true},
		{"action subpath case", typo.GitHubActions, "actons/tool/run", "actions/tool/Run", false},
		{"go owner", typo.Go, "github.com/shopsprint/decimal", "github.com/shopspring/decimal", true},
		{"go version preserved", typo.Go, "github.com/shopsprint/decimal/v2", "github.com/shopspring/decimal/v2", true},
		{"go different version", typo.Go, "github.com/shopsprint/decimal/v3", "github.com/shopspring/decimal/v2", false},
		{"go same owner", typo.Go, "github.com/shopspring/decimla", "github.com/shopspring/decimal", false},
		{"vanity path", typo.Go, "example.org/reqests", "example.org/requests", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			paths, err := typo.Compare(tt.candidate, tt.target, typo.Options{Ecosystem: tt.ecosystem})
			if err != nil {
				t.Fatal(err)
			}
			if (len(paths) != 0) != tt.match {
				t.Fatalf("paths = %v, want match %v", paths, tt.match)
			}
		})
	}
}

func TestInvalidNames(t *testing.T) {
	tests := []struct {
		ecosystem typo.Ecosystem
		name      string
	}{
		{typo.Generic, ""}, {typo.Generic, "\xff"}, {typo.Generic, "one\x01two"},
		{typo.PyPI, "trailing-"}, {typo.PyPI, "../requests"},
		{typo.PyPI, "Kelvin"}, {typo.Cargo, "Kelvin"},
		{typo.NPM, "@scope/"}, {typo.NPM, "Uppercase"}, {typo.NPM, strings.Repeat("a", 215)},
		{typo.Cargo, "1crate"}, {typo.Cargo, strings.Repeat("a", 65)},
		{typo.RubyGems, "gem/name"},
		{typo.GitHubActions, "actions"}, {typo.GitHubActions, "act--ions/checkout"},
		{typo.GitHubActions, "actions/checkout/../secret"},
		{typo.Go, "github.com/owner"}, {typo.Go, "github.com/owner//repo"},
		{typo.Go, "github.com/owner/repo@v1"},
	}
	for _, tt := range tests {
		_, err := typo.Generate(tt.name, typo.Options{Ecosystem: tt.ecosystem})
		if !errors.Is(err, typo.ErrInvalidName) {
			t.Errorf("%q %q: %v", tt.ecosystem, tt.name, err)
		}
	}
}

func TestOptions(t *testing.T) {
	tests := []typo.Options{
		{Ecosystem: "unknown"}, {MaxCandidates: -1}, {Algorithms: []typo.Algorithm{"unknown"}},
	}
	for _, options := range tests {
		_, err := typo.Generate("requests", options)
		if !errors.Is(err, typo.ErrInvalidOptions) {
			t.Errorf("%+v: %v", options, err)
		}
	}
	variants, err := typo.Generate("requests", typo.Options{Algorithms: []typo.Algorithm{}})
	if err != nil || len(variants) != 0 {
		t.Fatalf("empty selection: %v %v", variants, err)
	}
	selected := []typo.Algorithm{typo.Repetition, typo.Omission, typo.Repetition}
	original := slices.Clone(selected)
	if _, err = typo.Generate("requests", typo.Options{Algorithms: selected}); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(selected, original) {
		t.Fatal("options mutated")
	}
}

func TestLimit(t *testing.T) {
	variants, err := typo.Generate("one-two-three-four-five-six-seven-eight", typo.Options{Algorithms: []typo.Algorithm{typo.WordOrder}, MaxCandidates: 10})
	if !errors.Is(err, typo.ErrLimit) || variants != nil {
		t.Fatalf("got %v %v", variants, err)
	}
	paths, err := typo.Compare("same", "same", typo.Options{MaxCandidates: 1})
	if err != nil || len(paths) != 0 {
		t.Fatalf("identity: %v %v", paths, err)
	}
}

func TestGeneratedInvariants(t *testing.T) {
	for _, target := range []string{"requests", "a-a-a", "café", "日本語", ""} {
		options := typo.Options{MaxCandidates: 2000}
		first, err := typo.Generate(target, options)
		if target == "" {
			continue
		}
		if err != nil {
			t.Fatal(err)
		}
		second, err := typo.Generate(target, options)
		if err != nil || !reflect.DeepEqual(first, second) {
			t.Fatalf("nondeterministic result: %v", err)
		}
		previous := ""
		for _, variant := range first {
			if variant.Name <= previous || variant.Name == target {
				t.Fatalf("invalid order or identity: %q", variant.Name)
			}
			previous = variant.Name
			if len(variant.Paths) == 0 {
				t.Fatal("missing explanation")
			}
		}
	}
}

func TestDetector(t *testing.T) {
	owners := []string{"registry:alice"}
	detector, err := typo.NewDetector([]typo.Package{{Name: "requests", Owners: owners}, {Name: "flask"}}, typo.Options{Ecosystem: typo.PyPI})
	if err != nil {
		t.Fatal(err)
	}
	owners[0] = "registry:mallory"
	matches, err := detector.Check(typo.Package{Name: "rrequests", Owners: []string{"registry:mallory"}})
	if err != nil || len(matches) != 1 || matches[0].Target != "requests" {
		t.Fatalf("%v %v", matches, err)
	}
	matches[0].Paths[0][0] = "changed"
	again, err := detector.Check(typo.Package{Name: "rrequests"})
	if err != nil || again[0].Paths[0][0] == "changed" {
		t.Fatal("result aliases index")
	}
	for _, candidate := range []typo.Package{
		{Name: "Requests"}, {Name: "rrequests", Owners: []string{"registry:alice"}}, {Name: "unrelated"},
	} {
		matches, err = detector.Check(candidate)
		if err != nil || len(matches) != 0 {
			t.Fatalf("%+v: %v %v", candidate, matches, err)
		}
	}
	var group sync.WaitGroup
	for range 8 {
		group.Go(func() {
			if _, checkErr := detector.Check(typo.Package{Name: "flasl"}); checkErr != nil {
				t.Error(checkErr)
			}
		})
	}
	group.Wait()
}

func TestDetectorErrors(t *testing.T) {
	if _, err := typo.NewDetector([]typo.Package{{Name: "my_package"}, {Name: "My-Package"}}, typo.Options{Ecosystem: typo.PyPI}); err == nil {
		t.Fatal("duplicate canonical corpus name accepted")
	}
	if _, err := typo.NewDetector([]typo.Package{{Name: "requests"}}, typo.Options{MaxCandidates: 1}); !errors.Is(err, typo.ErrLimit) {
		t.Fatal(err)
	}
	if _, err := typo.NewDetector([]typo.Package{{Name: ""}}, typo.Options{}); !errors.Is(err, typo.ErrInvalidName) {
		t.Fatal(err)
	}
}

func FuzzGenerate(f *testing.F) {
	for _, seed := range []string{"requests", "café", "a-a-a-a", "日本語", "\xff"} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, target string) {
		if len(target) > 64 {
			t.Skip()
		}
		variants, err := typo.Generate(target, typo.Options{MaxCandidates: 1000})
		if err != nil {
			if !errors.Is(err, typo.ErrLimit) && !errors.Is(err, typo.ErrInvalidName) {
				t.Fatal(err)
			}
			return
		}
		previous := ""
		for _, variant := range variants {
			if variant.Name == target || variant.Name <= previous {
				t.Fatalf("invalid variant %q", variant.Name)
			}
			if _, err := typo.Generic.Normalize(variant.Name); err != nil {
				t.Fatal(err)
			}
			previous = variant.Name
		}
	})
}

func BenchmarkGenerate(b *testing.B) {
	for b.Loop() {
		if _, err := typo.Generate("requests", typo.Options{Ecosystem: typo.PyPI}); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkCompare(b *testing.B) {
	for b.Loop() {
		if _, err := typo.Compare("rrequests", "requests", typo.Options{Ecosystem: typo.PyPI}); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkCheck(b *testing.B) {
	detector, err := typo.NewDetector([]typo.Package{{Name: "requests"}, {Name: "flask"}, {Name: "numpy"}}, typo.Options{Ecosystem: typo.PyPI})
	if err != nil {
		b.Fatal(err)
	}
	b.ResetTimer()
	for b.Loop() {
		if _, err := detector.Check(typo.Package{Name: "rrequests"}); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkNewDetector(b *testing.B) {
	packages := make([]typo.Package, 100)
	for i := range packages {
		packages[i].Name = fmt.Sprintf("package%d", i)
	}
	b.ResetTimer()
	for b.Loop() {
		if _, err := typo.NewDetector(packages, typo.Options{Ecosystem: typo.PyPI}); err != nil {
			b.Fatal(err)
		}
	}
}
