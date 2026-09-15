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

func TestMergeComments(t *testing.T) {
	t.Run("preserves local fields on matched thread", func(t *testing.T) {
		existing := []comment{{
			Thread: "T1", Author: "rev", Source: "a.go:10",
			Status: statusWorking, Title: "rename x",
			FixRef: "abc123", Plan: "the plan", Reply: "the reply",
			Comment: "old body",
		}}
		fresh := []comment{{
			Thread: "T1", Author: "rev", Source: "a.go:12",
			Status: statusOpen, Comment: "edited body",
		}}
		got := mergeComments(existing, fresh)
		if len(got) != 1 {
			t.Fatalf("len = %d, want 1", len(got))
		}
		c := got[0]
		if c.Status != statusWorking || c.Title != "rename x" ||
			c.FixRef != "abc123" || c.Plan != "the plan" || c.Reply != "the reply" {
			t.Errorf("local fields not preserved: %+v", c)
		}
		if c.Comment != "edited body" || c.Source != "a.go:12" {
			t.Errorf("fresh upstream fields not taken: %+v", c)
		}
	})

	t.Run("empty existing status yields fresh open", func(t *testing.T) {
		existing := []comment{{Thread: "T1", Author: "rev", Plan: "p"}}
		fresh := []comment{{Thread: "T1", Author: "rev", Status: statusOpen}}
		got := mergeComments(existing, fresh)
		if got[0].Status != statusOpen {
			t.Errorf("status = %q, want open", got[0].Status)
		}
	})

	t.Run("same thread same author matches in order", func(t *testing.T) {
		existing := []comment{
			{Thread: "T1", Author: "rev", Comment: "first", Reply: "r1"},
			{Thread: "T1", Author: "rev", Comment: "second", Reply: "r2"},
		}
		fresh := []comment{
			{Thread: "T1", Author: "rev", Comment: "first", Status: statusOpen},
			{Thread: "T1", Author: "rev", Comment: "second", Status: statusOpen},
		}
		got := mergeComments(existing, fresh)
		if len(got) != 2 || got[0].Reply != "r1" || got[1].Reply != "r2" {
			t.Errorf("in-order matching failed: %+v", got)
		}
	})

	t.Run("resolved thread with local work kept closed", func(t *testing.T) {
		existing := []comment{{
			Thread: "gone", Author: "rev", Status: statusOpen, Reply: "draft",
		}}
		got := mergeComments(existing, nil)
		if len(got) != 1 || got[0].Status != statusClosed || got[0].Reply != "draft" {
			t.Errorf("local work not preserved as closed: %+v", got)
		}
	})

	t.Run("resolved thread without local work dropped", func(t *testing.T) {
		existing := []comment{{
			Thread: "gone", Author: "rev", Status: statusOpen, Comment: "nit",
		}}
		if got := mergeComments(existing, nil); len(got) != 0 {
			t.Errorf("bare comment kept: %+v", got)
		}
	})

	t.Run("new fresh comment appended", func(t *testing.T) {
		fresh := []comment{{Thread: "T2", Author: "rev", Status: statusOpen, Comment: "new"}}
		got := mergeComments(nil, fresh)
		if len(got) != 1 || got[0].Comment != "new" || got[0].Status != statusOpen {
			t.Errorf("fresh comment not appended: %+v", got)
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
