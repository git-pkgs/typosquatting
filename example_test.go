package typosquatting_test

import (
	"fmt"

	typo "github.com/git-pkgs/typosquatting"
)

func ExampleCompare() {
	paths, err := typo.Compare("rrequests", "requests", typo.Options{Ecosystem: typo.PyPI})
	if err != nil {
		panic(err)
	}
	fmt.Println(paths)
	// Output: [[addition] [adjacent_insertion] [repetition]]
}

func ExampleNewDetector() {
	detector, err := typo.NewDetector([]typo.Package{{Name: "actions/checkout"}}, typo.Options{Ecosystem: typo.GitHubActions})
	if err != nil {
		panic(err)
	}
	matches, err := detector.Check(typo.Package{Name: "acti0ns/checkout@main"})
	if err != nil {
		panic(err)
	}
	for _, match := range matches {
		fmt.Println(match.Candidate, match.Target, match.Paths)
	}
	// Output: acti0ns/checkout actions/checkout [[homoglyph]]
}
