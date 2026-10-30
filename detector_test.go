package typosquatting_test

import (
	"errors"
	"reflect"
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
