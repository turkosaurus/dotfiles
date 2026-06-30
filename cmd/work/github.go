package main

import "fmt"

// issue fetches issue details (title, status, etc.) from a GitHub issue URL.
// Will be backed by `gh issue view` or the GraphQL API.
func issue(url string) (Issue, error) {
	return Issue{}, fmt.Errorf("issue %q: not implemented", url)
}

// pr fetches PR details (title, mergeable state, comments) from a GitHub PR URL.
// Will be backed by `gh pr view` + queries/pr-threads.graphql for unresolved threads.
func pr(url string) (PR, error) {
	return PR{}, fmt.Errorf("pr %q: not implemented", url)
}
