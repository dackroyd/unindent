package unindent

import (
	"go/ast"
	"go/build"
	"go/build/constraint"
	"go/parser"
	"go/token"
	"go/types"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/ctrlflow"
	"golang.org/x/tools/go/cfg"
	"golang.org/x/tools/go/types/typeutil"
)

// buildDependent is the fact that a function never returns only in some builds. The runtime's racerelease, for
// example, is a stub that throws in builds without -race, declared in race0.go, and race.go declares the one that
// returns.
//
// The test is a heuristic that needs no other build to be loaded: a function never returns only in some builds when
// it is declared in a file with build constraints and also in one of the package's files that this build leaves out,
// or when it would return if the never-returning functions it calls that never return only in some builds did. A
// counterpart that also never returns still counts, as files left out are not type checked.
type buildDependent struct{}

func (*buildDependent) AFact() {}

func (*buildDependent) String() string { return "buildDependent" }

// buildAnalyzer finds the functions that never return only in some builds, and exports the fact for them, so that
// unindent knows them in every package that calls them. It is separate from unindent, which declares no facts, so the
// driver only runs this cheap part on every dependency.
var buildAnalyzer = &analysis.Analyzer{
	Name:       "unindentbuild",
	Doc:        "Finds functions that never return only in some builds",
	Requires:   []*analysis.Analyzer{ctrlflow.Analyzer},
	Run:        runBuild,
	FactTypes:  []analysis.Fact{new(buildDependent)},
	ResultType: reflect.TypeFor[*buildDependents](),
	URL:        "https://github.com/dackroyd/unindent",
}

// buildDependents reports which functions never return only in some builds, in the package analysed and those it
// imports.
type buildDependents struct {
	pass  *analysis.Pass
	local map[*types.Func]bool
}

// Has reports whether fn never returns only in some builds.
func (d *buildDependents) Has(fn *types.Func) bool {
	if fn.Pkg() == d.pass.Pkg {
		return d.local[fn]
	}

	return d.pass.ImportObjectFact(fn, new(buildDependent))
}

func runBuild(pass *analysis.Pass) (any, error) {
	cfgs, _ := pass.ResultOf[ctrlflow.Analyzer].(*ctrlflow.CFGs)
	d := &buildDependents{pass: pass, local: map[*types.Func]bool{}}

	// The functions declared in the package that never return, and whether their file has build constraints.
	type decl struct {
		fn          *types.Func
		fd          *ast.FuncDecl
		constrained bool
		// replaced is set when a file left out of this build declares the function too.
		replaced bool
	}

	var decls []decl

	for _, f := range pass.Files {
		constrained := hasConstraints(pass.Fset, f)

		for _, d := range f.Decls {
			fd, ok := d.(*ast.FuncDecl)
			if !ok {
				continue
			}

			fn, ok := pass.TypesInfo.Defs[fd.Name].(*types.Func)
			if !ok || !cfgs.NoReturn(fn) {
				continue
			}

			decls = append(decls, decl{fn: fn, fd: fd, constrained: constrained})
		}
	}

	// mayReturn is ctrlflow's view of whether call may return, or with lenient set, the view of a build in which the
	// calls that never return only in some builds return.
	mayReturn := func(lenient bool) func(*ast.CallExpr) bool {
		return func(call *ast.CallExpr) bool {
			if id, ok := ast.Unparen(call.Fun).(*ast.Ident); ok {
				if b, ok := pass.TypesInfo.Uses[id].(*types.Builtin); ok && b.Name() == "panic" {
					return false
				}
			}

			fn := typeutil.StaticCallee(pass.TypesInfo, call)

			return fn == nil || !cfgs.NoReturn(fn) || (lenient && d.Has(fn))
		}
	}

	// ctrlflow knows some functions never return by name, such as syscall.Exit and runtime.fatalthrow, in every build,
	// whatever their body says, and assembly functions have none. Only the others can depend on the build.
	var (
		candidates []decl
		otherDecls map[string]bool
	)

	for _, dc := range decls {
		if dc.fd.Body == nil || !cfg.New(dc.fd.Body, mayReturn(false)).NoReturn() {
			continue
		}

		// A function in a file with build constraints never returns only in some builds when a file left out of
		// this build declares it too, as race.go declares the racerelease of race0.go. Without such a counterpart,
		// every build with a call to the function has this declaration of it.
		if dc.constrained {
			if otherDecls == nil {
				otherDecls = otherBuildDecls(pass)
			}

			dc.replaced = otherDecls[declName(dc.fd)]
		}

		candidates = append(candidates, dc)
	}

	// Any other function never returns only in some builds when it could return if the calls that never return only
	// in some builds did: it calls racerelease, but not panic or os.Exit too. Each one found can make others depend
	// on it, in any order, so the search repeats until it finds no more.
	for changed := true; changed; {
		changed = false

		for _, dc := range candidates {
			if !d.local[dc.fn] && (dc.replaced || !cfg.New(dc.fd.Body, mayReturn(true)).NoReturn()) {
				d.local[dc.fn] = true
				changed = true
			}
		}
	}

	for fn := range d.local {
		pass.ExportObjectFact(fn, new(buildDependent))
	}

	return d, nil
}

// hasConstraints reports whether f is only part of some builds: it has a //go:build or // +build line, a name that
// go/build matches against GOOS and GOARCH, such as x_linux.go, or uses cgo.
func hasConstraints(fset *token.FileSet, f *ast.File) bool {
	for _, imp := range f.Imports {
		if imp.Path.Value == `"C"` {
			return true
		}
	}

	for _, g := range f.Comments {
		if g.Pos() > f.Package {
			break
		}

		for _, c := range g.List {
			if constraint.IsGoBuild(c.Text) || constraint.IsPlusBuild(c.Text) {
				return true
			}
		}
	}

	return constrainedName(filepath.Base(fset.File(f.Pos()).Name()))
}

// noPlatform matches no GOOS or GOARCH, so go/build only matches files whose names don't select one. Its files are
// read as empty, so their build lines don't count.
var noPlatform = build.Context{
	GOOS:     "none",
	GOARCH:   "none",
	Compiler: "gc",
	OpenFile: func(string) (io.ReadCloser, error) { return io.NopCloser(strings.NewReader("package p\n")), nil },
}

// constrainedName reports whether go/build only builds a file named name for some GOOS or GOARCH.
func constrainedName(name string) bool {
	ok, err := noPlatform.MatchFile("", name)

	return err == nil && !ok
}

// otherBuildDecls returns the names, as declName gives them, of the functions declared in the package's Go files that
// this build leaves out. Files of another package, such as a generator in package main, are skipped.
func otherBuildDecls(pass *analysis.Pass) map[string]bool {
	readFile := pass.ReadFile
	if readFile == nil {
		readFile = os.ReadFile
	}

	names := map[string]bool{}
	fset := token.NewFileSet()

	for _, name := range pass.IgnoredFiles {
		if filepath.Ext(name) != ".go" {
			continue
		}

		src, err := readFile(name)
		if err != nil {
			continue
		}

		f, _ := parser.ParseFile(fset, name, src, parser.SkipObjectResolution)
		if f == nil || f.Name == nil || f.Name.Name != pass.Pkg.Name() {
			continue
		}

		for _, d := range f.Decls {
			if fd, ok := d.(*ast.FuncDecl); ok {
				names[declName(fd)] = true
			}
		}
	}

	return names
}

// declName returns the name of a function, or of a method qualified by its receiver's type: "f" or "T.m".
func declName(fd *ast.FuncDecl) string {
	if fd.Recv == nil || len(fd.Recv.List) == 0 {
		return fd.Name.Name
	}

	t := fd.Recv.List[0].Type
	for {
		switch x := ast.Unparen(t).(type) {
		case *ast.StarExpr:
			t = x.X
		case *ast.IndexExpr:
			t = x.X
		case *ast.IndexListExpr:
			t = x.X
		case *ast.Ident:
			return x.Name + "." + fd.Name.Name
		default:
			return fd.Name.Name
		}
	}
}
