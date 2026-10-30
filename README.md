# typosquatting

A Go library for generating package-name variants and detecting potential typosquats against a supplied corpus. Supports PyPI, npm, RubyGems, Cargo, Go, and GitHub Actions using only the standard library.

## Install

Requires Go 1.26 or later.

```sh
go get github.com/git-pkgs/typosquatting
```

## Usage

```go
paths, err := typosquatting.Compare("rrequests", "requests", typosquatting.Options{
    Ecosystem: typosquatting.PyPI,
})
if err != nil {
    return err
}
fmt.Println(paths)
// [[addition] [adjacent_insertion] [repetition]]
```

Each path describes an alternative way to produce the candidate from the target. The paths above each contain one step: inserting `r` before the name, inserting a key adjacent to `e`, or repeating the initial `r`. Combined edits are not generated.

`Compare` checks a candidate against one target. `Generate` returns distinct variants sorted by name, with all matching paths retained.

### Check against a corpus

`NewDetector` builds an index for checking many candidates against the same corpus:

```go
detector, err := typosquatting.NewDetector([]typosquatting.Package{
    {Name: "actions/checkout"},
}, typosquatting.Options{Ecosystem: typosquatting.GitHubActions})
if err != nil {
    return err
}
matches, err := detector.Check(typosquatting.Package{Name: "acti0ns/checkout@main"})
if err != nil {
    return err
}
for _, match := range matches {
    fmt.Println(match.Candidate, match.Target, match.Paths)
}
// acti0ns/checkout actions/checkout [[homoglyph]]
```

Callers supply the corpus and any verified registry owner IDs. Exact corpus members and pairs with a shared owner are excluded. The index is safe for concurrent checks and copies input metadata and returned paths. Its memory use grows with the number of generated variants.

## Algorithms

The module includes the 16 algorithm categories from [andrew/typosquatting](https://github.com/andrew/typosquatting), including keyboard substitutions, homoglyphs, delimiter changes, word permutations, and common affixes. `Algorithms()` lists them. A nil `Options.Algorithms` selects all algorithms; an empty slice selects none.

`MaxCandidates` limits raw generation attempts per target, including invalid names and duplicates. The default is 10,000; exceeding it returns `ErrLimit` with no partial results.

## Supported ecosystems

| Ecosystem | Profile | Comparison rules |
|-----------|---------|------------------|
| PyPI | `PyPI` | Case insensitive; runs of `-`, `_`, and `.` are equivalent |
| npm | `NPM` | Scoped packages vary the scope and preserve the package name |
| RubyGems | `RubyGems` | Case sensitive; `-` and `_` remain distinct |
| Cargo | `Cargo` | Case insensitive; `-` and `_` are equivalent |
| Go | `Go` | GitHub module paths vary the owner and preserve the member path |
| GitHub Actions | `GitHubActions` | Varies the owner; removes refs and preserves subdirectory case |
| Generic | `Generic` | Raw UTF-8 names with case preserved; the default profile |

Go module version suffixes remain part of the name. Vanity Go module paths have no inferred owner boundary, so each path component can vary independently.

Validation covers a conservative subset of package-name syntax. It does not check registry availability or reserved names. A spelling match describes similarity; callers determine what to do with it using package metadata and their own policy.

## Development

Run the tests and race detector:

```sh
go test -race ./...
```

Run the fuzz target:

```sh
go test -fuzz=FuzzGenerate -fuzztime=30s
```

Run the benchmarks:

```sh
go test -run '^$' -bench . -benchmem
```

## Related projects

- [ecosyste-ms/typosquatting-dataset](https://github.com/ecosyste-ms/typosquatting-dataset): Package-name pairs with target mappings, classifications, and source attribution for evaluating typosquatting detection.
- [andrew/typosquatting](https://github.com/andrew/typosquatting): Ruby library for generation and ecosystem rules; source of the spelling tables.
- [typomania](https://github.com/rustfoundation/typomania): Rust library for corpus matching and shared-owner filtering.

## License

[MIT](LICENSE).
