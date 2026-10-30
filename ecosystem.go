package typosquatting

import (
	"fmt"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Ecosystem selects package spelling and namespace rules. Generic preserves case
// and accepts any nonempty UTF-8 name without control characters.
type Ecosystem string

const (
	Generic       Ecosystem = ""
	PyPI          Ecosystem = "pypi"
	NPM           Ecosystem = "npm"
	RubyGems      Ecosystem = "gem"
	Cargo         Ecosystem = "cargo"
	Go            Ecosystem = "golang"
	GitHubActions Ecosystem = "github_actions"
)

const (
	maxNPMName     = 214
	maxCrateName   = 64
	maxGitHubOwner = 39
	maxGitHubRepo  = 100
)

var (
	pypiName       = regexp.MustCompile(`^[a-zA-Z0-9]([a-zA-Z0-9._-]*[a-zA-Z0-9])?$`)
	npmName        = regexp.MustCompile(`^(@[a-z0-9~-][a-z0-9._~-]*/)?[a-z0-9~-][a-z0-9._~-]*$`)
	gemName        = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_-]*$`)
	crateName      = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9_-]*$`)
	ownerName      = regexp.MustCompile(`^[a-zA-Z0-9]([a-zA-Z0-9-]*[a-zA-Z0-9])?$`)
	repoName       = regexp.MustCompile(`^[a-zA-Z0-9_-][a-zA-Z0-9._-]*$`)
	pathElement    = regexp.MustCompile(`^[a-zA-Z0-9_-][a-zA-Z0-9._-]*$`)
	pypiDelimiters = regexp.MustCompile(`[-_.]+`)
)

func (e Ecosystem) supported() bool {
	switch e {
	case Generic, PyPI, NPM, RubyGems, Cargo, Go, GitHubActions:
		return true
	}
	return false
}

// Normalize returns the comparison name or ErrInvalidName. Validation checks
// spelling syntax, not availability, reserved names, or registry ownership.
// GitHub Actions refs are removed; Go module version suffixes are preserved.
func (e Ecosystem) Normalize(name string) (string, error) {
	if !e.supported() {
		return "", fmt.Errorf("%w: ecosystem %q", ErrInvalidOptions, e)
	}
	if name == "" || !utf8.ValidString(name) || strings.IndexFunc(name, unicode.IsControl) >= 0 {
		return "", fmt.Errorf("%w: %q", ErrInvalidName, name)
	}
	canonical, valid := e.canonical(name)
	if !valid {
		return "", fmt.Errorf("%w for %s: %q", ErrInvalidName, e, name)
	}
	return canonical, nil
}

func (e Ecosystem) canonical(name string) (string, bool) {
	switch e {
	case PyPI:
		lower := strings.ToLower(name)
		return pypiDelimiters.ReplaceAllString(lower, "-"), pypiName.MatchString(name)
	case NPM:
		return name, len(name) <= maxNPMName && npmName.MatchString(name)
	case RubyGems:
		return name, gemName.MatchString(name)
	case Cargo:
		lower := strings.ToLower(name)
		return strings.ReplaceAll(lower, "_", "-"), len(name) <= maxCrateName && crateName.MatchString(name)
	case GitHubActions:
		return canonicalAction(name)
	case Go:
		return name, validModulePath(name)
	default:
		return name, true
	}
}

func canonicalAction(name string) (string, bool) {
	name, _, _ = strings.Cut(name, "@")
	parts := strings.Split(name, "/")
	if len(parts) < 2 || !validOwner(parts[0]) || len(parts[1]) > maxGitHubRepo || !repoName.MatchString(parts[1]) {
		return "", false
	}
	for _, part := range parts[2:] {
		if !pathElement.MatchString(part) {
			return "", false
		}
	}
	parts[0], parts[1] = strings.ToLower(parts[0]), strings.ToLower(parts[1])
	return strings.Join(parts, "/"), true
}

func validOwner(name string) bool {
	return len(name) <= maxGitHubOwner && ownerName.MatchString(name) && !strings.Contains(name, "--")
}

func validModulePath(name string) bool {
	parts := strings.Split(name, "/")
	if len(parts) < 2 || !strings.Contains(parts[0], ".") {
		return false
	}
	for _, part := range parts {
		if !pathElement.MatchString(part) || strings.HasSuffix(part, ".") {
			return false
		}
	}
	if parts[0] == "github.com" {
		return len(parts) >= 3 && validOwner(parts[1])
	}
	return true
}

type namePart struct{ text, prefix, suffix string }

func (e Ecosystem) parts(name string) []namePart {
	if e == NPM && strings.HasPrefix(name, "@") {
		scope, member, _ := strings.Cut(name[1:], "/")
		return []namePart{{text: scope, prefix: "@", suffix: "/" + member}}
	}
	if e == GitHubActions {
		owner, member, _ := strings.Cut(name, "/")
		return []namePart{{text: owner, suffix: "/" + member}}
	}
	if e == Go {
		return moduleParts(name)
	}
	return []namePart{{text: name}}
}

func moduleParts(name string) []namePart {
	if strings.HasPrefix(name, "github.com/") {
		owner, member, _ := strings.Cut(strings.TrimPrefix(name, "github.com/"), "/")
		return []namePart{{text: owner, prefix: "github.com/", suffix: "/" + member}}
	}
	// Vanity module paths have no generally inferable owner boundary.
	parts := strings.Split(name, "/")
	result := make([]namePart, 0, len(parts))
	offset := 0
	for _, part := range parts {
		result = append(result, namePart{text: part, prefix: name[:offset], suffix: name[offset+len(part):]})
		offset += len(part) + 1
	}
	return result
}
