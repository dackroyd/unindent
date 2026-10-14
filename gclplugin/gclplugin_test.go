package gclplugin_test

import (
	"testing"

	"github.com/golangci/plugin-module-register/register"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	_ "github.com/dackroyd/unindent/gclplugin"
)

func TestSettings(t *testing.T) {
	t.Parallel()

	newPlugin, err := register.GetPlugin("unindent")
	require.NoError(t, err)

	for _, tc := range []struct {
		name     string
		settings any
		want     map[string]string
		wantErr  string
	}{
		{
			name: "none",
			want: map[string]string{
				"else-after-exit": "true", "else-if-after-exit": "true", "exit-in-else": "true",
				"if-wrapping-rest": "true", "if-wrapping-rest-min-statements": "3",
				"if-alone-in-if": "true", "if-alone-in-if-max-operands": "2",
			},
		},
		{name: "empty", settings: map[string]any{}, want: map[string]string{"exit-in-else": "true"}},
		{
			name:     "yaml",
			settings: map[string]any{"exit-in-else": false, "if-wrapping-rest-min-statements": 4},
			want: map[string]string{
				"exit-in-else": "false", "if-wrapping-rest-min-statements": "4", "else-after-exit": "true",
			},
		},
		{
			name:     "json",
			settings: map[string]any{"if-alone-in-if-max-operands": 3.0},
			want:     map[string]string{"if-alone-in-if-max-operands": "3"},
		},
		{
			name:     "toml",
			settings: map[string]any{"if-alone-in-if-max-operands": int64(3)},
			want:     map[string]string{"if-alone-in-if-max-operands": "3"},
		},
		{
			name:     "unknown",
			settings: map[string]any{"exit-in-els": false},
			wantErr: `unknown setting "exit-in-els": the settings are else-after-exit, else-if-after-exit, ` +
				`exit-in-else, if-alone-in-if, if-alone-in-if-max-operands, if-wrapping-rest, ` +
				`if-wrapping-rest-min-statements`,
		},
		{
			name:     "bool as number",
			settings: map[string]any{"exit-in-else": 0},
			wantErr:  `setting "exit-in-else" must be true or false, not 0`,
		},
		{
			name:     "bool as string",
			settings: map[string]any{"exit-in-else": "false"},
			wantErr:  `setting "exit-in-else" must be true or false, not the string "false"`,
		},
		{
			name:     "number as bool",
			settings: map[string]any{"if-wrapping-rest-min-statements": true},
			wantErr:  `setting "if-wrapping-rest-min-statements" must be a whole number, not true`,
		},
		{
			name:     "number as string",
			settings: map[string]any{"if-alone-in-if-max-operands": "3"},
			wantErr:  `setting "if-alone-in-if-max-operands" must be a whole number, not the string "3"`,
		},
		{
			name:     "fraction",
			settings: map[string]any{"if-alone-in-if-max-operands": 2.5},
			wantErr:  `setting "if-alone-in-if-max-operands" must be a whole number, not 2.5`,
		},
		{
			name:     "below the minimum",
			settings: map[string]any{"if-alone-in-if-max-operands": 1},
			wantErr:  `setting "if-alone-in-if-max-operands": must be at least 2, not 1`,
		},
		{
			name:     "list",
			settings: []any{"exit-in-else"},
			wantErr:  "the settings must map setting names to values, not []interface {}",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			p, err := newPlugin(tc.settings)
			if tc.wantErr != "" {
				require.EqualError(t, err, tc.wantErr)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, register.LoadModeTypesInfo, p.GetLoadMode())

			analyzers, err := p.BuildAnalyzers()
			require.NoError(t, err)
			require.Len(t, analyzers, 1)
			assert.Equal(t, "unindent", analyzers[0].Name)

			for name, want := range tc.want {
				assert.Equal(t, want, analyzers[0].Flags.Lookup(name).Value.String(), name)
			}
		})
	}
}

// TestSettingsPerPlugin checks that each plugin has its own flags, as golangci-lint may build more than one.
func TestSettingsPerPlugin(t *testing.T) {
	t.Parallel()

	newPlugin, err := register.GetPlugin("unindent")
	require.NoError(t, err)

	off, err := newPlugin(map[string]any{"exit-in-else": false})
	require.NoError(t, err)

	def, err := newPlugin(nil)
	require.NoError(t, err)

	offAnalyzers, err := off.BuildAnalyzers()
	require.NoError(t, err)

	defAnalyzers, err := def.BuildAnalyzers()
	require.NoError(t, err)

	assert.Equal(t, "false", offAnalyzers[0].Flags.Lookup("exit-in-else").Value.String())
	assert.Equal(t, "true", defAnalyzers[0].Flags.Lookup("exit-in-else").Value.String())
}
