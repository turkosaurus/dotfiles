package main

import (
	"reflect"
	"testing"
)

func TestParseProjectURL(t *testing.T) {
	cases := []struct {
		name       string
		in         string
		wantOwner  string
		wantNumber int
		wantErr    bool
	}{
		{"org", "https://github.com/orgs/getlantern/projects/127", "getlantern", 127, false},
		{"user", "https://github.com/users/jay/projects/3", "jay", 3, false},
		{"trailing slash", "https://github.com/orgs/x/projects/9/", "x", 9, false},
		{"missing number", "https://github.com/orgs/x/projects/", "", 0, true},
		{"garbage", "not-a-url", "", 0, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gotOwner, gotNumber, err := parseProjectURL(tc.in)
			if tc.wantErr {
				if err == nil {
					t.Errorf("expected error, got (%q, %d)", gotOwner, gotNumber)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if gotOwner != tc.wantOwner || gotNumber != tc.wantNumber {
				t.Errorf("got (%q, %d), want (%q, %d)", gotOwner, gotNumber, tc.wantOwner, tc.wantNumber)
			}
		})
	}
}

func TestBuildStatusLookup(t *testing.T) {
	in := map[string][]string{
		"open":    {"In Sprint", "Todo"},
		"working": {"In Progress"},
		"waiting": {"In Review"},
	}
	got := buildStatusLookup(in)
	want := map[string]statusKind{
		"in sprint":   statusOpen,
		"todo":        statusOpen,
		"in progress": statusWorking,
		"in review":   statusWaiting,
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestBuildStatusLookupCaseAndSpace(t *testing.T) {
	in := map[string][]string{
		"  OPEN  ":  {"  In Sprint  "},
		"WORKING":   {"In progress"},
	}
	got := buildStatusLookup(in)
	if got["in sprint"] != statusOpen {
		t.Errorf("case-insensitive remote lookup failed: %v", got)
	}
	if got["in progress"] != statusWorking {
		t.Errorf("case-insensitive remote lookup failed: %v", got)
	}
}

func TestSortedIgnoredCols(t *testing.T) {
	in := map[string]int{
		"Backlog":      20,
		"Epics":        5,
		"Needs Triage": 5,
		"Done":         1,
	}
	got := sortedIgnoredCols(in)
	// Highest count first; ties break alphabetically.
	want := []string{"Backlog×20", "Epics×5", "Needs Triage×5", "Done×1"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

