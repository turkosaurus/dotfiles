package main

import (
	"reflect"
	"testing"
	"time"
)

func TestPickMergePrimaryTiers(t *testing.T) {
	// Three picks: a task, a plain worktree, a worktree with a PR. Primary
	// should be the worktree+PR (tier 3), then plain worktree (2), then task.
	task := inventoryItem{Task: &plan{Path: "/w/t/open/1.toml", Title: "t1"}}
	plainBranch := inventoryItem{Worktree: &worktree{Path: "/w/repo/br1"}}
	branchWithPR := inventoryItem{Worktree: &worktree{Path: "/w/repo/br2"}}

	newer := time.Now()
	older := newer.Add(-1 * time.Hour)

	cache := map[string]plan{
		task.key():         {Path: task.Task.Path, mtime: newer},
		plainBranch.key():  {Path: plainBranch.Worktree.Path, mtime: newer},
		branchWithPR.key(): {Path: branchWithPR.Worktree.Path, mtime: older, PRs: []PR{{URL: "https://x"}}},
	}
	picks := []inventoryItem{task, plainBranch, branchWithPR}
	primary, others := pickMergePrimary(picks, cache)
	if primary.key() != branchWithPR.key() {
		t.Errorf("primary should be branch+PR; got %+v", primary)
	}
	if len(others) != 2 {
		t.Errorf("expected 2 others, got %d", len(others))
	}
}

func TestPickMergePrimaryTiebreaksByMtime(t *testing.T) {
	// Two tasks at the same tier; newer wins.
	older := inventoryItem{Task: &plan{Path: "/w/t/open/1.toml", Title: "older"}}
	newer := inventoryItem{Task: &plan{Path: "/w/t/open/2.toml", Title: "newer"}}
	now := time.Now()
	cache := map[string]plan{
		older.key(): {Path: older.Task.Path, mtime: now.Add(-2 * time.Hour)},
		newer.key(): {Path: newer.Task.Path, mtime: now},
	}
	primary, _ := pickMergePrimary([]inventoryItem{older, newer}, cache)
	if primary.key() != newer.key() {
		t.Errorf("newer task should win the tie; got %+v", primary)
	}
}

func TestMergePlanFieldsUnions(t *testing.T) {
	dst := plan{
		Tasks:  []string{"a", "b"},
		Issues: []Issue{{URL: "https://issue/1", Title: "one"}},
		PRs:    []PR{{URL: "https://pr/A", Title: "A"}},
	}
	src := plan{
		Tasks: []string{"b", "c"}, // "b" is a dupe, should be skipped
		Issues: []Issue{
			{URL: "https://issue/1", Title: "still one"}, // dupe URL, skipped
			{URL: "https://issue/2", Title: "two"},
		},
		PRs: []PR{
			{URL: "https://pr/A", Title: "dupe"}, // URL dupe, skipped
			{URL: "https://pr/B", Title: "B"},
		},
	}
	mergePlanFields(&dst, src)

	if !reflect.DeepEqual(dst.Tasks, []string{"a", "b", "c"}) {
		t.Errorf("tasks union failed: %v", dst.Tasks)
	}
	if len(dst.Issues) != 2 {
		t.Errorf("issues union failed: %v", dst.Issues)
	}
	if dst.Issues[0].URL != "https://issue/1" || dst.Issues[1].URL != "https://issue/2" {
		t.Errorf("issue order/URLs wrong: %v", dst.Issues)
	}
	if len(dst.PRs) != 2 {
		t.Errorf("PRs union failed: %v", dst.PRs)
	}
	if dst.PRs[0].URL != "https://pr/A" || dst.PRs[1].URL != "https://pr/B" {
		t.Errorf("PR order/URLs wrong: %v", dst.PRs)
	}
}

func TestMergePlanFieldsSlackFallback(t *testing.T) {
	// Slack: primary empty, src has one → adopt.
	dst := plan{}
	src := plan{Slack: slack{URL: "https://slack/x"}}
	mergePlanFields(&dst, src)
	if dst.Slack.URL != "https://slack/x" {
		t.Errorf("expected slack adopted; got %+v", dst.Slack)
	}

	// Primary set, src has one → keep primary.
	dst2 := plan{Slack: slack{URL: "https://slack/primary"}}
	src2 := plan{Slack: slack{URL: "https://slack/other"}}
	mergePlanFields(&dst2, src2)
	if dst2.Slack.URL != "https://slack/primary" {
		t.Errorf("expected primary slack kept; got %+v", dst2.Slack)
	}
}

func TestMergePlanFieldsPRUnionDropsEmpty(t *testing.T) {
	// Empty stubs (URL "") should be dropped from both dst and src.
	dst := plan{PRs: []PR{{}, {URL: "https://a"}}}
	src := plan{PRs: []PR{{}, {URL: "https://b"}}}
	mergePlanFields(&dst, src)
	urls := make([]string, 0, len(dst.PRs))
	for _, p := range dst.PRs {
		urls = append(urls, p.URL)
	}
	want := []string{"https://a", "https://b"}
	if !reflect.DeepEqual(urls, want) {
		t.Errorf("PR union: got %v, want %v", urls, want)
	}
}
