// Package gclplugin registers unindent as the golangci-lint module plugin "unindent", for a golangci-lint built with
// "golangci-lint custom". See docs/golangci-lint.md.
package gclplugin

import (
	"flag"
	"fmt"
	"maps"
	"math"
	"slices"
	"strings"

	"github.com/golangci/plugin-module-register/register"
	"golang.org/x/tools/go/analysis"

	"github.com/dackroyd/unindent"
)

func init() {
	register.Plugin("unindent", New)
}

type plugin struct {
	analyzer *analysis.Analyzer
}

// New returns the plugin for the settings under linters.settings.custom.unindent.settings, which map the analyzer's
// flags to their values: true or false for a check, a whole number for a check's setting.
func New(settings any) (register.LinterPlugin, error) {
	an := unindent.NewAnalyzer()
	if err := setFlags(an, settings); err != nil {
		return nil, err
	}

	return plugin{analyzer: an}, nil
}

func (p plugin) BuildAnalyzers() ([]*analysis.Analyzer, error) {
	return []*analysis.Analyzer{p.analyzer}, nil
}

// GetLoadMode asks for type information: the checks find the calls that never return, in other packages too, from
// the types and facts.
func (plugin) GetLoadMode() string {
	return register.LoadModeTypesInfo
}

func setFlags(an *analysis.Analyzer, settings any) error {
	if settings == nil {
		return nil
	}

	values, ok := settings.(map[string]any)
	if !ok {
		return fmt.Errorf("the settings must map setting names to values, not %T", settings)
	}

	for _, name := range slices.Sorted(maps.Keys(values)) {
		f := an.Flags.Lookup(name)
		if f == nil {
			return fmt.Errorf("unknown setting %q: the settings are %s", name, flagNames(an))
		}

		value, err := flagValue(f, values[name])
		if err != nil {
			return fmt.Errorf("setting %q must be %w", name, err)
		}

		if err := an.Flags.Set(name, value); err != nil {
			return fmt.Errorf("setting %q: %w", name, err)
		}
	}

	return nil
}

// flagValue returns v as the text f's Set takes. Settings arrive as YAML, JSON or TOML decode them: a bool, or an
// int, a float64 or an int64.
func flagValue(f *flag.Flag, v any) (string, error) {
	if b, ok := f.Value.(interface{ IsBoolFlag() bool }); ok && b.IsBoolFlag() {
		if _, ok := v.(bool); !ok {
			return "", fmt.Errorf("true or false, not %s", describe(v))
		}

		return fmt.Sprint(v), nil
	}

	switch n := v.(type) {
	case int, int64, uint64:
		return fmt.Sprint(n), nil
	case float64:
		if n == math.Trunc(n) && math.Abs(n) < math.MaxInt32 {
			return fmt.Sprint(int64(n)), nil
		}
	}

	return "", fmt.Errorf("a whole number, not %s", describe(v))
}

func describe(v any) string {
	if s, ok := v.(string); ok {
		return fmt.Sprintf("the string %q", s)
	}

	return fmt.Sprint(v)
}

func flagNames(an *analysis.Analyzer) string {
	var names []string

	an.Flags.VisitAll(func(f *flag.Flag) { names = append(names, f.Name) })

	return strings.Join(names, ", ")
}
