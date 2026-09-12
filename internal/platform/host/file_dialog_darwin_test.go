//go:build darwin

package host

import "testing"

func TestAppleScriptTypeList(t *testing.T) {
	for _, tc := range []struct {
		in   []string
		want string
	}{
		{nil, ""},
		{[]string{}, ""},
		{[]string{".st", "msa", " stx "}, `{"st", "msa", "stx"}`},
		{[]string{"", "  "}, ""},
	} {
		if got := appleScriptTypeList(tc.in); got != tc.want {
			t.Errorf("appleScriptTypeList(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}
