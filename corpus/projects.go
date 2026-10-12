package main

// goVersions are the Go releases that build and analyse the corpus, one for each release the Go team supports, oldest
// first. Each one's standard library is part of the default set, so each is pinned to a patch release, and has results
// of its own.
var goVersions = []string{"go1.26.9", "go1.27.0"}

// latestGo is the latest release in goVersions, which checks the large set and runs the comparison with revive.
// golangci-lint v2.13.1, which has the revive release the comparison documents, cannot read the export data of Go 1.27.2
// and later.
func latestGo() string {
	return goVersions[len(goVersions)-1]
}

// golangciLint is the golangci-lint release that runs revive for the comparison.
const golangciLint = "github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.13.1"

type project struct {
	// Name identifies the project in the results.
	Name string
	// Repo and Commit locate the source. A project without a Repo is the standard library of the Go release in use, which is
	// reported on but not fixed, as it lives in the read-only module cache.
	Repo, Commit string
	// Date is the date of Commit, and Description says what the project is, for the documentation.
	Date, Description string
	// Batched analyses each top-level directory of each module separately, rather than the module's ./... at once,
	// so that the packages loaded at once, and the memory they take, stay small.
	Batched bool
	// Tests are the packages whose tests must still pass once the fixes are applied.
	Tests []string
}

// corpusSet is a list of projects checked together, with results of its own.
type corpusSet struct {
	Name string
	// Releases are the Go releases the set is checked with, from goVersions.
	Releases []string
	// Doc is the documentation page generated from the results of the latest release, relative to the corpus
	// directory, if any.
	Doc      string
	Projects []project
}

// defaultSet is the set that "check" uses unless -set says otherwise, and that CI checks on every change.
const defaultSet = "default"

var sets = []corpusSet{
	{
		// Well-known projects with few dependencies, as the check's memory and disk grow with them, and fast tests,
		// chosen so that together they cover every finding kind and every reason a fix is withheld. Every go.mod
		// supports the oldest release in goVersions, so each project has one commit for every release.
		Name:     defaultSet,
		Releases: goVersions,
		Projects: []project{
			{
				Name:        "stdlib",
				Description: "The Go standard library and toolchain, `std` and `cmd`.",
				// Analysing std or cmd as one ./... loads all of it at once. One top-level directory at a time keeps
				// the memory down, with the same findings.
				Batched: true,
			},
			{
				Name:        "nats-server",
				Repo:        "https://github.com/nats-io/nats-server",
				Commit:      "64dda80fdd87b41b2afcdc5faf5d846db6c189b0",
				Date:        "2026-10-09",
				Description: "A messaging server.",
				// The full suite takes over 10 minutes, peaks at 10 GB and has timing-dependent failures. These
				// packages take seconds.
				Tests: []string{
					"./conf/...", "./internal/...", "./server/avl/...", "./server/ats/...", "./server/gsl/...",
					"./server/pse/...", "./server/stree/...", "./server/thw/...",
				},
			},
			{
				Name:        "esbuild",
				Repo:        "https://github.com/evanw/esbuild",
				Commit:      "f6058f8364fe7ab91ca57a83e02577ed74c9cae4",
				Date:        "2026-08-09",
				Description: "A JavaScript bundler and minifier.",
				Tests:       []string{"./..."},
			},
			{
				Name:        "x-net",
				Repo:        "https://github.com/golang/net",
				Commit:      "c1d31df183e51b1075a2fd670d6f6f4efb871773",
				Date:        "2026-10-09",
				Description: "Supplementary networking packages, such as HTTP/2 and QUIC.",
				Tests:       []string{"./..."},
			},
			{
				Name:        "x-crypto",
				Repo:        "https://github.com/golang/crypto",
				Commit:      "3b414e3e8284a71c817c4d1009acdd5d3a464ff9",
				Date:        "2026-10-11",
				Description: "Supplementary cryptography packages, such as SSH.",
				Tests:       []string{"./..."},
			},
			{
				Name:        "x-mod",
				Repo:        "https://github.com/golang/mod",
				Commit:      "b5136e5ba156da612a1efc6e49b55988a134a5e0",
				Date:        "2026-10-09",
				Description: "Packages for Go modules.",
				Tests:       []string{"./..."},
			},
			{
				Name:        "compress",
				Repo:        "https://github.com/klauspost/compress",
				Commit:      "1974577b19ef1edd312803ff61b11cf86f870b33",
				Date:        "2026-10-12",
				Description: "Compression libraries: zstd, S2, flate and others.",
				Tests:       []string{"./..."},
			},
			{
				Name:        "age",
				Repo:        "https://github.com/FiloSottile/age",
				Commit:      "b74dce4cdbe35b5e5f66c06d9612b72f89028758",
				Date:        "2026-08-29",
				Description: "A file encryption tool and library.",
				Tests:       []string{"./..."},
			},
			{
				Name:        "encoding",
				Repo:        "https://github.com/segmentio/encoding",
				Commit:      "7d5a25dbc5da13aed3cb047a127e4d0e96f536fb",
				Date:        "2026-03-10",
				Description: "Fast JSON and Protocol Buffers encoders.",
				Tests:       []string{"./..."},
			},
			{
				Name:        "goccy-yaml",
				Repo:        "https://github.com/goccy/go-yaml",
				Commit:      "edee2f91616c6d73112a13e7c0dbde72ce938877",
				Date:        "2026-04-07",
				Description: "A YAML parser.",
				Tests:       []string{"./..."},
			},
		},
	},
	{
		// Large codebases, checked weekly and before each release. They take up to half an hour and 31 GB of disk with
		// -clean-cache, as they have many dependencies.
		Name:     "large",
		Releases: []string{latestGo()},
		Doc:      "../docs/projects.md",
		Projects: []project{
			{
				Name:        "tools",
				Repo:        "https://github.com/golang/tools",
				Commit:      "16517d951552611e7bd358c48ee1051fd0e57b0c",
				Date:        "2026-10-10",
				Description: "Go's analysis and refactoring tools, and the gopls language server.",
				Tests:       []string{"./go/...", "./internal/...", "./refactor/..."},
			},
			{
				Name:        "grpc-go",
				Repo:        "https://github.com/grpc/grpc-go",
				Commit:      "8c59ee6356bf92d0a5819b0cdbd3118876783028",
				Date:        "2026-10-10",
				Description: "The Go implementation of gRPC.",
			},
			{
				Name:        "terraform",
				Repo:        "https://github.com/hashicorp/terraform",
				Commit:      "d8e73fcbd9bb9d7c7ab38242eaccef5bfe88e270",
				Date:        "2026-10-09",
				Description: "An infrastructure as code tool.",
			},
			{
				Name:        "hugo",
				Repo:        "https://github.com/gohugoio/hugo",
				Commit:      "c9f6e4d4abc3645cb46824c870e4e1b4c098b765",
				Date:        "2026-10-11",
				Description: "A static site generator.",
			},
			{
				Name:        "prometheus",
				Repo:        "https://github.com/prometheus/prometheus",
				Commit:      "fffc15f5b5a6c3ed50282d307e96db0e5f25bcb2",
				Date:        "2026-10-11",
				Description: "A monitoring system and time series database.",
			},
		},
	},
}

// findSet returns the set named name.
func findSet(name string) (corpusSet, bool) {
	for _, s := range sets {
		if s.Name == name {
			return s, true
		}
	}

	return corpusSet{}, false
}
