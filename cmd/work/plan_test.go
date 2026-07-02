package main

import (
	"reflect"
	"strings"
	"testing"
)

func TestUpgradeLegacyPR(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "single [pr] header rewritten",
			in:   "title = 'x'\n[pr]\nurl = ''\n",
			want: "title = 'x'\n[[pr]]\nurl = ''\n",
		},
		{
			name: "already-array file is untouched",
			in:   "title = 'x'\n[[pr]]\nurl = 'https://x'\n",
			want: "title = 'x'\n[[pr]]\nurl = 'https://x'\n",
		},
		{
			name: "[pr] inside a multi-line string is preserved when [[pr]] already exists elsewhere",
			in:   "tasks = ['''\nsee [pr] block\n''']\n[[pr]]\nurl = ''\n",
			want: "tasks = ['''\nsee [pr] block\n''']\n[[pr]]\nurl = ''\n",
		},
		{
			name: "unrelated line looking like [pr.comment] is untouched",
			in:   "[pr]\n[pr.comment]\nurl = ''\n",
			// Only the standalone [pr] header matches; the sub-table stays put.
			want: "[[pr]]\n[pr.comment]\nurl = ''\n",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := string(upgradeLegacyPR([]byte(tc.in)))
			if got != tc.want {
				t.Errorf("mismatch\n got:  %q\n want: %q", got, tc.want)
			}
		})
	}
}

func TestUpsertPR(t *testing.T) {
	base := []PR{
		{URL: "https://github.com/a/1", Title: "one"},
		{URL: "https://github.com/a/2", Title: "two"},
	}

	t.Run("replaces on url match", func(t *testing.T) {
		got := upsertPR(base, PR{URL: "https://github.com/a/1", Title: "updated"})
		if got[0].Title != "updated" {
			t.Errorf("expected replacement; got %+v", got)
		}
		if len(got) != 2 {
			t.Errorf("expected len 2, got %d", len(got))
		}
	})

	t.Run("appends on no match", func(t *testing.T) {
		got := upsertPR(base, PR{URL: "https://github.com/a/3", Title: "three"})
		if len(got) != 3 || got[2].Title != "three" {
			t.Errorf("expected append; got %+v", got)
		}
	})

	t.Run("empty slice appends", func(t *testing.T) {
		got := upsertPR(nil, PR{URL: "https://x", Title: "x"})
		if len(got) != 1 || got[0].URL != "https://x" {
			t.Errorf("expected 1-element result; got %+v", got)
		}
	})
}

func TestFirstPR(t *testing.T) {
	if firstPR(plan{}).URL != "" {
		t.Errorf("firstPR on empty plan should return zero PR")
	}
	p := plan{PRs: []PR{{URL: "https://x"}, {URL: "https://y"}}}
	if firstPR(p).URL != "https://x" {
		t.Errorf("firstPR should return index 0; got %+v", firstPR(p))
	}
}

func TestCompactPlan(t *testing.T) {
	in := plan{
		Issues: []Issue{
			{URL: ""},
			{URL: "https://a", Title: "a"},
			{URL: ""},
			{URL: "https://b", Title: "b"},
		},
		PRs: []PR{
			{URL: ""},
			{URL: "https://pr/1"},
			{URL: ""},
		},
	}
	out := compactPlan(in)
	if len(out.Issues) != 2 || out.Issues[0].URL != "https://a" || out.Issues[1].URL != "https://b" {
		t.Errorf("issues not compacted correctly: %+v", out.Issues)
	}
	if len(out.PRs) != 1 || out.PRs[0].URL != "https://pr/1" {
		t.Errorf("PRs not compacted correctly: %+v", out.PRs)
	}
	// The compact should not mutate the caller's slices.
	if len(in.Issues) != 4 {
		t.Errorf("compactPlan mutated caller's Issues")
	}
}

func TestUpgradeLegacyPRRegexAnchor(t *testing.T) {
	// Guard against `[[pr]]` on the same line as text that could match the
	// single-header regex. The (?m)^\[pr\]$ anchor should reject anything
	// but a bare line.
	in := []byte("foo = '[pr]'\n[pr]\n")
	got := string(upgradeLegacyPR(in))
	if !strings.Contains(got, "[[pr]]") {
		t.Errorf("expected header rewritten: %q", got)
	}
	if !strings.Contains(got, "foo = '[pr]'") {
		t.Errorf("string literal should not be touched: %q", got)
	}
}

func TestPlanRoundtripStructural(t *testing.T) {
	// Round-trip a plan with all fields populated to guard against silent
	// schema drift (adding a struct field without a toml tag, e.g.).
	in := plan{
		Title: "t",
		Tasks: []string{"a", "b"},
		Issues: []Issue{
			{URL: "https://x", Title: "x"},
		},
		PRs: []PR{
			{URL: "https://pr", Title: "pr"},
		},
	}
	compact := compactPlan(in)
	if !reflect.DeepEqual(compact.Tasks, in.Tasks) {
		t.Errorf("tasks changed: %+v vs %+v", compact.Tasks, in.Tasks)
	}
	if len(compact.Issues) != 1 || compact.Issues[0].URL != "https://x" {
		t.Errorf("issue lost during compact: %+v", compact.Issues)
	}
}
