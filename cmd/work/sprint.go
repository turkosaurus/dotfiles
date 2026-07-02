package main

import (
	"fmt"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/cli/go-gh/v2/pkg/api"
	"github.com/pterm/pterm"
)

// sprintFetchResult carries the items pre-fetched from GitHub plus the
// config context needed to reconcile them. Used by runSyncAll to overlap
// the sprint fetch with the worktree loop.
type sprintFetchResult struct {
	items    []projectItem
	config   config
	err      error
	disabled bool // config.sprint.project_url is empty
}

// fetchSprint reads config, resolves assignees, and fetches project items.
// Split out from reconcileSprint so runSyncAll can overlap the fetch with
// the worktree loop. onFirstReply, when set, is invoked once the very
// first search page returns — useful for driving a 3-tick progress bar
// where the first tick means "server started responding".
func fetchSprint(onFirstReply func()) sprintFetchResult {
	c, err := loadConfig()
	if err != nil {
		return sprintFetchResult{err: fmt.Errorf("sprint: %w", err)}
	}
	if c.Sprint.ProjectURL == "" {
		return sprintFetchResult{config: c, disabled: true}
	}
	owner, num, err := parseProjectURL(c.Sprint.ProjectURL)
	if err != nil {
		return sprintFetchResult{err: fmt.Errorf("sprint: parse project_url: %w", err)}
	}
	assignees, err := resolveAssignees(c.Sprint.Assignees)
	if err != nil {
		return sprintFetchResult{err: fmt.Errorf("sprint: resolve assignees: %w", err)}
	}
	if len(assignees) == 0 {
		pterm.Warning.Println("sprint: config.sprint.assignees is empty — refusing to fetch all board items")
		return sprintFetchResult{config: c, disabled: true}
	}
	if len(buildStatusLookup(c.Sprint.StatusFields)) == 0 {
		pterm.Warning.Println("sprint: config.sprint.status_fields is empty — nothing to sync")
		return sprintFetchResult{config: c, disabled: true}
	}

	items, err := fetchSprintItems(owner, num, assignees, onFirstReply)
	if err != nil {
		return sprintFetchResult{err: fmt.Errorf("sprint: %w", err)}
	}
	return sprintFetchResult{items: items, config: c}
}

// reconcileSprint applies the fetched items to local plans: creates new
// tasks for untracked URLs and updates status on tracked ones. All disk
// writes happen here, so callers should ensure any concurrent goroutine
// touching the same plans has completed before invoking.
//
// Output is buffered into out.lines (already colored via pterm) so the
// caller can flush after any concurrent progress bar has stopped, avoiding
// cursor collisions.
type sprintOutput struct {
	lines []string
	stats reconcileStats
}

// reconcileStats is the numeric summary of a sprint plan. Rendered as a
// table by sprintOutput.flush() — always labeled "sprint (proposed)"
// since it precedes the confirm.
type reconcileStats struct {
	created, updated, skipped, blocked, total int
	ignoredByCol                              map[string]int
	// disabled means no sync happened (no project_url, empty status_fields, etc.)
	disabled bool
}

// info/success/warn/errline append pre-styled buffered lines. info and
// success respect quietMode (skipped when set); warn and err always
// surface.
func (out *sprintOutput) info(format string, a ...any) {
	if quietMode {
		return
	}
	out.lines = append(out.lines, pterm.Info.Sprintfln(format, a...))
}
func (out *sprintOutput) success(format string, a ...any) {
	if quietMode {
		return
	}
	out.lines = append(out.lines, pterm.Success.Sprintfln(format, a...))
}
func (out *sprintOutput) warn(format string, a ...any) {
	out.lines = append(out.lines, pterm.Warning.Sprintfln(format, a...))
}
func (out *sprintOutput) errline(format string, a ...any) {
	out.lines = append(out.lines, pterm.Error.Sprintfln(format, a...))
}

// sprintAction is a deferred mutation queued by planSprint. Silent actions
// (stamp-only refreshes of project.url/project.status) apply without
// contributing to the confirm-prompt count — the user only needs to
// approve status changes and creations, not metadata refreshes.
type sprintAction struct {
	label  string
	do     func() error
	silent bool
}

// planSprint walks the fetched items, builds preview output + stats, and
// returns the actions that would carry out the reconcile. No disk writes
// happen inside — callers gate execution behind a confirm.
func planSprint(r sprintFetchResult) (sprintOutput, []sprintAction, error) {
	var out sprintOutput
	if r.err != nil {
		return out, nil, r.err
	}
	if r.disabled {
		out.stats.disabled = true
		return out, nil, nil
	}
	c := r.config
	items := r.items
	lookup := buildStatusLookup(c.Sprint.StatusFields)
	tracked, err := trackedPlans()
	if err != nil {
		return out, nil, fmt.Errorf("sprint: scan tracked: %w", err)
	}

	var actions []sprintAction
	var created, updated, skipped, blocked int
	ignoredByCol := map[string]int{}
	for _, it := range items {
		if it.Content.URL == "" {
			continue
		}
		// GitHub-closed issues override the column mapping. If we have a
		// local plan tracking it, force target = closed (and let the block
		// caveat handle plans with pending tasks[]). If we don't have a
		// local plan, skip — no point creating a task for something already
		// done on the server.
		var target statusKind
		var ok bool
		if it.Closed {
			target, ok = statusClosed, true
		} else {
			target, ok = lookup[strings.ToLower(strings.TrimSpace(it.Status))]
			if !ok {
				col := strings.TrimSpace(it.Status)
				if col == "" {
					col = "(no status)"
				}
				ignoredByCol[col]++
				log.Debug("sprint: ignored (unlisted column)",
					log.Args("column", col, "title", it.Content.Title, "url", it.Content.URL))
				continue
			}
		}
		title := it.Content.Title
		if title == "" {
			title = it.Title
		}

		if p, ok := tracked[it.Content.URL]; ok {
			label := planLabel(*p)
			stampNeeded := issueNeedsStamp(*p, it.Content.URL, c.Sprint.ProjectURL, it.Status)
			updateIssueProject(p, it.Content.URL, c.Sprint.ProjectURL, it.Status)
			if p.Status == target {
				// Closed items that stay closed are done — no summary
				// noise, no stamp write. Everything else may still need
				// a silent stamp if project fields drifted.
				if target == statusClosed {
					continue
				}
				if stampNeeded {
					pCopy := *p
					actions = append(actions, sprintAction{
						label:  label,
						silent: true,
						do:     func() error { return writePlan(pCopy) },
					})
				}
				skipped++
				continue
			}
			if target == statusClosed && len(p.Tasks) > 0 {
				out.warn("would update %s (kept open: %d task(s) remain)", label, len(p.Tasks))
				blocked++
				continue
			}
			note := fmt.Sprintf("%s → %s", p.Status, target)
			out.success("would update %s (%s)", label, note)
			pRef, tgt := p, target
			actions = append(actions, sprintAction{
				label: label,
				do:    func() error { return updatePlanStatus(pRef, tgt) },
			})
			updated++
			continue
		}

		// Never create a fresh task for an already-closed issue — pointless
		// noise. (Existing tracked plans still get updated to closed above.)
		if it.Closed {
			skipped++
			continue
		}
		createNote := fmt.Sprintf("new → %s", target)
		out.success("would update %s (%s)", title, createNote)
		urlCopy, titleCopy, tgt, projURL, itStatus := it.Content.URL, title, target, c.Sprint.ProjectURL, it.Status
		actions = append(actions, sprintAction{
			label: titleCopy,
			do: func() error {
				np, err := newTask(titleCopy, tgt)
				if err != nil {
					return err
				}
				np.Issues = append(np.Issues, Issue{
					URL:   urlCopy,
					Title: titleCopy,
					Project: IssueProject{
						URL:    projURL,
						Status: itStatus,
					},
				})
				return writePlan(np)
			},
		})
		created++
	}

	out.stats = reconcileStats{
		created:      created,
		updated:      updated,
		skipped:      skipped,
		blocked:      blocked,
		total:        len(items),
		ignoredByCol: ignoredByCol,
	}
	return out, actions, nil
}

// applySprint runs each queued action. Errors are buffered as error lines
// on out via out.errline so the caller can flush them after any progress
// indicator has stopped. Returns the count of failed applications so the
// caller can propagate a non-zero exit code.
func applySprint(actions []sprintAction, out *sprintOutput) int {
	failed := 0
	for _, a := range actions {
		if err := a.do(); err != nil {
			out.errline("apply %s (error: %v)", a.label, err)
			failed++
		}
	}
	return failed
}

// countUserFacingActions returns how many actions are "loud" — status
// changes and creations. Silent actions (metadata stamps) are excluded so
// the confirm prompt matches the numbers shown in the sprint table.
func countUserFacingActions(actions []sprintAction) int {
	n := 0
	for _, a := range actions {
		if !a.silent {
			n++
		}
	}
	return n
}

// issueNeedsStamp reports whether the linked issue in p already has the
// project.url and project.status we're about to stamp. Used to short-
// circuit no-op writes on subsequent syncs after the initial stamp.
func issueNeedsStamp(p plan, url, projectURL, status string) bool {
	for _, i := range p.Issues {
		if i.URL == url {
			return i.Project.URL != projectURL || i.Project.Status != status
		}
	}
	return false
}

// flushLines prints the buffered per-item output. Callers should invoke
// this after any concurrent progress indicator has stopped, then decide
// separately whether to render the summary table via renderTable.
func (s sprintOutput) flushLines() {
	for _, l := range s.lines {
		fmt.Print(l)
	}
}

// renderTable prints the "sprint (proposed)" summary. No-op under quiet
// mode or when nothing was actually processed.
func (s sprintOutput) renderTable() {
	if quietMode || s.stats.disabled || s.stats.total == 0 {
		return
	}
	renderSprintTable(s.stats)
}

// renderSprintTable draws a compact metric | count table with the ignored
// columns indented underneath. Nothing is printed when no items were
// processed (stats.total == 0).
func renderSprintTable(s reconcileStats) {
	ignored := 0
	for _, n := range s.ignoredByCol {
		ignored += n
	}
	// Table is always "proposed" — it renders before the confirm, showing
	// what the plan would do. Dry-run just stops after showing it.
	title := "sprint (proposed)"
	rows := pterm.TableData{
		{title, ""},
		{"created", strconv.Itoa(s.created)},
		{"updated", strconv.Itoa(s.updated)},
		{"unchanged", strconv.Itoa(s.skipped)},
		{"kept open (has tasks)", strconv.Itoa(s.blocked)},
		{"ignored", strconv.Itoa(ignored)},
	}
	// Per-column ignored breakdown is noise for the common case; only
	// include it under -v. Count still shows in the "ignored" row above.
	if verboseMode {
		for _, name := range sortedIgnoredCols(s.ignoredByCol) {
			if idx := strings.LastIndex(name, "×"); idx > 0 {
				rows = append(rows, []string{"  " + name[:idx], name[idx+len("×"):]})
			} else {
				rows = append(rows, []string{"  " + name, ""})
			}
		}
	}
	rows = append(rows, []string{"total", strconv.Itoa(s.total)})
	if err := pterm.DefaultTable.WithHasHeader().WithData(rows).Render(); err != nil {
		log.Debug("sprint table render", log.Args("err", err))
	}
}

// planLabel returns a human-friendly identifier for a plan — its title if
// set, falling back to the relative path (matches how tasks are labeled
// on creation and mirrors the picker's Name column).
func planLabel(p plan) string {
	if p.Title != "" {
		return p.Title
	}
	return relPath(p.Path)
}

// updateIssueProject stamps or refreshes the project.url + project.status
// on the issue in p matching url. No-op when the issue isn't found (caller
// invariant: url came from p.Issues in the first place).
func updateIssueProject(p *plan, url, projectURL, status string) {
	for i := range p.Issues {
		if p.Issues[i].URL == url {
			p.Issues[i].Project.URL = projectURL
			p.Issues[i].Project.Status = status
			return
		}
	}
}

// sortedIgnoredCols renders `[col×N]` for each unlisted column, sorted by
// count descending so the largest source of ignored items shows first.
func sortedIgnoredCols(counts map[string]int) []string {
	type kv struct {
		k string
		v int
	}
	pairs := make([]kv, 0, len(counts))
	for k, v := range counts {
		pairs = append(pairs, kv{k, v})
	}
	sort.SliceStable(pairs, func(i, j int) bool {
		if pairs[i].v != pairs[j].v {
			return pairs[i].v > pairs[j].v
		}
		return pairs[i].k < pairs[j].k
	})
	out := make([]string, 0, len(pairs))
	for _, p := range pairs {
		out = append(out, fmt.Sprintf("%s×%d", p.k, p.v))
	}
	return out
}

// resolveAssignees expands the config.sprint.assignees list. "@me" becomes
// the authenticated GitHub user (via `gh api user`); other entries pass
// through unchanged. Returns nil for an empty/nil input — the caller
// interprets that as "no filter".
func resolveAssignees(cfg []string) ([]string, error) {
	if len(cfg) == 0 {
		return nil, nil
	}
	var out []string
	var meCache string
	for _, a := range cfg {
		a = strings.TrimSpace(a)
		if a == "" {
			continue
		}
		if a == "@me" {
			if meCache == "" {
				login, err := currentGitHubLogin()
				if err != nil {
					return nil, err
				}
				meCache = login
			}
			out = append(out, meCache)
			continue
		}
		out = append(out, a)
	}
	return out, nil
}


// updatePlanStatus rewrites p's status. For task plans the underlying file is
// moved into the correct ~/w/t/<status>/ directory; for worktree plans the
// file stays put and just gets rewritten.
func updatePlanStatus(p *plan, target statusKind) error {
	if isTaskPath(p.Path) {
		np, err := moveTask(*p, target)
		if err != nil {
			return err
		}
		*p = np
		return nil
	}
	p.Status = target
	return writePlan(*p)
}

// isTaskPath reports whether path lives under the task root (~/w/t/…).
func isTaskPath(p string) bool {
	return strings.HasPrefix(p, defaultTaskDir+"/")
}

// projectItem mirrors the subset of `gh project item-list --format json` we
// consume. Extra fields are ignored.
type projectItem struct {
	Title     string             `json:"title"`
	Status    string             `json:"status"`
	Closed    bool               `json:"closed"` // GitHub issue/PR state, not project column
	Assignees []string           `json:"assignees"`
	Content   projectItemContent `json:"content"`
}

type projectItemContent struct {
	URL   string `json:"url"`
	Title string `json:"title"`
	Type  string `json:"type"`
}

// currentGitHubLogin queries `{ viewer { login } }` and returns the
// authenticated user's handle. Used to expand "@me" in config.sprint.assignees.
func currentGitHubLogin() (string, error) {
	client, err := api.DefaultGraphQLClient()
	if err != nil {
		return "", fmt.Errorf("gh client: %w", err)
	}
	var resp struct {
		Viewer struct{ Login string }
	}
	if err := client.Do(`query { viewer { login } }`, nil, &resp); err != nil {
		if rl := asGhRateLimit(err); rl != nil {
			return "", rl
		}
		return "", fmt.Errorf("viewer: %w", err)
	}
	return resp.Viewer.Login, nil
}

// projectURLRe matches https://github.com/{orgs|users}/<owner>/projects/<n>
var projectURLRe = regexp.MustCompile(`^https://github\.com/(?:orgs|users)/([^/]+)/projects/(\d+)/?$`)

func parseProjectURL(u string) (owner string, number int, err error) {
	m := projectURLRe.FindStringSubmatch(strings.TrimSpace(u))
	if m == nil {
		return "", 0, fmt.Errorf("unrecognized project URL %q (want https://github.com/orgs/<owner>/projects/<n>)", u)
	}
	n, err := strconv.Atoi(m[2])
	if err != nil {
		return "", 0, fmt.Errorf("bad project number %q: %w", m[2], err)
	}
	return m[1], n, nil
}

// searchNode mirrors the subset of the GraphQL response we consume for one
// search hit. Extracted so the paginated fetch loop can reuse it without
// re-declaring the anonymous shape.
type searchNode struct {
	URL          string
	Title        string
	Closed       bool // true if the issue/PR is closed on GitHub
	Assignees    struct{ Nodes []struct{ Login string } }
	ProjectItems struct {
		Nodes []struct {
			Project          struct{ Number int }
			FieldValueByName struct{ Name string }
		}
	}
}

// sprintSearchResponse wraps searchNode with the pagination cursor GitHub
// returns on the `search` connection.
type sprintSearchResponse struct {
	Search struct {
		PageInfo struct {
			HasNextPage bool
			EndCursor   string
		}
		Nodes []searchNode
	}
}

// fetchSprintItems runs the sprint GraphQL query per assignee, scoped
// server-side by `assignee:X project:owner/N`. Follows the search cursor
// until `hasNextPage` is false so we never truncate at 100. Results are
// dedup'd by URL across assignees.
//
// onFirstReply, if non-nil, is invoked exactly once after the first page
// of the first assignee returns successfully — used by callers to advance
// a progress bar to "server has replied" before all pagination completes.
func fetchSprintItems(owner string, num int, assignees []string, onFirstReply func()) ([]projectItem, error) {
	client, err := api.DefaultGraphQLClient()
	if err != nil {
		return nil, fmt.Errorf("gh client: %w", err)
	}
	seen := map[string]bool{}
	var out []projectItem
	firstReplyFired := false
	for _, a := range assignees {
		q := fmt.Sprintf("assignee:%s project:%s/%d", a, owner, num)
		cursor := ""
		for {
			var resp sprintSearchResponse
			vars := map[string]interface{}{"q": q}
			if cursor != "" {
				vars["after"] = cursor
			} else {
				vars["after"] = nil
			}
			if err := client.Do(sprintQuery, vars, &resp); err != nil {
				if rl := asGhRateLimit(err); rl != nil {
					return nil, rl
				}
				return nil, fmt.Errorf("search %q: %w", q, err)
			}
			if !firstReplyFired && onFirstReply != nil {
				onFirstReply()
				firstReplyFired = true
			}
			for _, n := range resp.Search.Nodes {
				if n.URL == "" || seen[n.URL] {
					continue
				}
				var status string
				for _, pi := range n.ProjectItems.Nodes {
					if pi.Project.Number == num {
						status = pi.FieldValueByName.Name
						break
					}
				}
				logins := make([]string, 0, len(n.Assignees.Nodes))
				for _, ass := range n.Assignees.Nodes {
					logins = append(logins, ass.Login)
				}
				out = append(out, projectItem{
					Title:     n.Title,
					Status:    status,
					Closed:    n.Closed,
					Assignees: logins,
					Content:   projectItemContent{URL: n.URL, Title: n.Title},
				})
				seen[n.URL] = true
			}
			if !resp.Search.PageInfo.HasNextPage {
				break
			}
			// Guard against an infinite loop if the server ever returns
			// HasNextPage=true with an empty cursor (spec violation, but
			// cheap insurance).
			if resp.Search.PageInfo.EndCursor == "" {
				return nil, fmt.Errorf("search %q: server reported more pages but no cursor", q)
			}
			cursor = resp.Search.PageInfo.EndCursor
		}
	}
	return out, nil
}

// trackedPlans walks every plan under ~/w and returns a URL → *plan map,
// covering worktree plans and task plans alike. When the same URL appears in
// more than one plan, the last one wins — not ideal but sprint-sync is a
// best-effort reconciliation and duplicates are the user's problem to resolve
// (via `work rm` or `work convert`).
func trackedPlans() (map[string]*plan, error) {
	set := map[string]*plan{}
	scan := func(pattern string) error {
		matches, err := filepath.Glob(pattern)
		if err != nil {
			return err
		}
		for _, m := range matches {
			p, err := readPlan(m)
			if err != nil {
				continue
			}
			pc := p // copy so pointer is stable per match
			for _, iss := range p.Issues {
				if iss.URL == "" {
					continue
				}
				if prev, dup := set[iss.URL]; dup && prev.Path != pc.Path {
					pterm.Warning.Printfln("issue %s linked in two plans: %s and %s",
						iss.URL, relPath(prev.Path), relPath(pc.Path))
				}
				set[iss.URL] = &pc
			}
		}
		return nil
	}
	if err := scan(filepath.Join(defaultWorkDir, "*", "*", planFileName)); err != nil {
		return nil, err
	}
	if err := scan(filepath.Join(defaultTaskDir, "*", "*.toml")); err != nil {
		return nil, err
	}
	return set, nil
}

// buildStatusLookup inverts config.sprint.status_fields (local → [remote])
// into a lowercased remote-column → local-statusKind map so the item loop can
// resolve columns in constant time. Columns not present in the returned map
// are treated as "ignore" (never touched).
func buildStatusLookup(fields map[string][]string) map[string]statusKind {
	out := map[string]statusKind{}
	for local, remotes := range fields {
		s := statusKind(strings.ToLower(strings.TrimSpace(local)))
		for _, r := range remotes {
			out[strings.ToLower(strings.TrimSpace(r))] = s
		}
	}
	return out
}
