package main

import (
	"embed"
	"fmt"
	"net/url"
	"path"
	"strconv"
	"strings"

	"github.com/cli/go-gh/v2/pkg/api"
)

//go:embed queries/*.graphql
var queries embed.FS

var (
	issueQuery      = query("issue.graphql")
	prQuery         = query("pr-threads.graphql")
	prByBranchQuery = query("pr-by-branch.graphql")
)

func query(name string) string {
	data, err := queries.ReadFile(path.Join("queries", name))
	if err != nil {
		panic(fmt.Errorf("read embedded query %q: %w", name, err))
	}
	return string(data)
}

// ghPR and IssueRef are lightweight pointers returned alongside fetches.
// Not persisted to TOML — used only for resolving linkage during sync.
type ghPR struct {
	Number  int
	Title   string
	URL     string
	IsDraft bool
}

type ghIssue struct {
	Number int
	Title  string
	URL    string
	Closed bool
}

// prBranchInfo is the lightweight PR summary returned by prForBranch.
type prBranchInfo struct {
	URL      string
	State    string // OPEN, CLOSED, MERGED
	MergedAt string // ISO timestamp; empty when not merged
}

// prForBranch returns the most-recent PR summary for the given head branch,
// or nil if none exists.
func prForBranch(owner, repo, branch string) (*prBranchInfo, error) {
	client, err := api.DefaultGraphQLClient()
	if err != nil {
		return nil, fmt.Errorf("gh client: %w", err)
	}
	var resp struct {
		Repository struct {
			PullRequests struct {
				Nodes []struct {
					URL      string
					State    string
					MergedAt string
				}
			}
		}
	}
	vars := map[string]interface{}{
		"owner": owner, "repo": repo, "branch": branch,
	}
	if err := client.Do(prByBranchQuery, vars, &resp); err != nil {
		return nil, fmt.Errorf("query pr-by-branch %s/%s@%s: %w", owner, repo, branch, err)
	}
	if len(resp.Repository.PullRequests.Nodes) == 0 {
		return nil, nil
	}
	n := resp.Repository.PullRequests.Nodes[0]
	return &prBranchInfo{URL: n.URL, State: n.State, MergedAt: n.MergedAt}, nil
}

// parseGHURL splits https://github.com/owner/repo/{issues|pull}/N into parts.
func parseGHURL(raw string) (owner, repo string, number int, err error) {
	u, err := url.Parse(raw)
	if err != nil {
		return "", "", 0, fmt.Errorf("parse url %q: %w", raw, err)
	}
	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	if len(parts) < 4 {
		return "", "", 0, fmt.Errorf("bad url %q: expected owner/repo/issues|pull/N", raw)
	}
	owner, repo = parts[0], parts[1]
	number, err = strconv.Atoi(parts[3])
	if err != nil {
		return "", "", 0, fmt.Errorf("bad number in %q: %w", raw, err)
	}
	return owner, repo, number, nil
}

func issue(rawURL string) (Issue, []ghPR, error) {
	owner, repo, number, err := parseGHURL(rawURL)
	if err != nil {
		return Issue{}, nil, fmt.Errorf("issue: %w", err)
	}

	client, err := api.DefaultGraphQLClient()
	if err != nil {
		return Issue{}, nil, fmt.Errorf("gh client: %w", err)
	}

	var resp struct {
		Repository struct {
			Issue struct {
				Title                          string
				URL                            string
				Closed                         bool
				ClosedByPullRequestsReferences struct {
					Nodes []struct {
						Number  int
						Title   string
						URL     string
						IsDraft bool
					}
				}
			}
		}
	}
	vars := map[string]interface{}{
		"owner": owner, "repo": repo, "number": number,
	}
	if err := client.Do(issueQuery, vars, &resp); err != nil {
		return Issue{}, nil, fmt.Errorf("query issue %s: %w", rawURL, err)
	}

	i := resp.Repository.Issue
	var linked []ghPR
	for _, n := range i.ClosedByPullRequestsReferences.Nodes {
		linked = append(linked, ghPR{
			Number: n.Number, Title: n.Title, URL: n.URL, IsDraft: n.IsDraft,
		})
	}
	return Issue{
		Title:  i.Title,
		URL:    i.URL,
		Closed: i.Closed,
	}, linked, nil
}

func pr(rawURL string) (PR, []ghIssue, error) {
	owner, repo, number, err := parseGHURL(rawURL)
	if err != nil {
		return PR{}, nil, fmt.Errorf("pr: %w", err)
	}

	client, err := api.DefaultGraphQLClient()
	if err != nil {
		return PR{}, nil, fmt.Errorf("gh client: %w", err)
	}

	var resp struct {
		Repository struct {
			PullRequest struct {
				Title                   string
				URL                     string
				Mergeable               string
				IsDraft                 bool
				ClosingIssuesReferences struct {
					Nodes []struct {
						Number int
						Title  string
						URL    string
						Closed bool
					}
				}
				ReviewThreads struct {
					Nodes []struct {
						ID         string
						IsResolved bool
						Comments   struct {
							Nodes []struct {
								Author struct{ Login string }
								Body   string
								Path   string
								Line   int
								URL    string
							}
						}
					}
				}
			}
		}
	}
	vars := map[string]interface{}{
		"owner": owner, "repo": repo, "number": number,
	}
	if err := client.Do(prQuery, vars, &resp); err != nil {
		return PR{}, nil, fmt.Errorf("query pr %s: %w", rawURL, err)
	}

	p := resp.Repository.PullRequest
	mergeable := strings.ToLower(p.Mergeable)
	if p.IsDraft {
		mergeable = "draft"
	}

	var comments []comment
	for _, t := range p.ReviewThreads.Nodes {
		if t.IsResolved {
			continue
		}
		for _, c := range t.Comments.Nodes {
			comments = append(comments, comment{
				Status:  statusOpen,
				Source:  fmt.Sprintf("%s:%d", c.Path, c.Line),
				Author:  c.Author.Login,
				Thread:  t.ID,
				Comment: c.Body,
			})
		}
	}

	var closes []ghIssue
	for _, n := range p.ClosingIssuesReferences.Nodes {
		closes = append(closes, ghIssue{
			Number: n.Number, Title: n.Title, URL: n.URL, Closed: n.Closed,
		})
	}

	return PR{
		Title:     p.Title,
		URL:       p.URL,
		Mergeable: mergeable,
		Comments:  comments,
	}, closes, nil
}
