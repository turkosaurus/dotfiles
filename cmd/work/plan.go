package main

import (
	"fmt"
	"os"
	"time"

	"github.com/pelletier/go-toml/v2"
)

type statusKind string

const (
	statusOpen    statusKind = "open"
	statusWaiting statusKind = "waiting"
	statusWorking statusKind = "working"
	statusClosed  statusKind = "closed"

	planFileName             = "plan.toml"
	planFileMode os.FileMode = 0o644
)

type plan struct {
	Title  string         `toml:"title"`
	Status statusKind     `toml:"status"`
	Due    toml.LocalDate `toml:"due"`
	Tasks  []string       `toml:"tasks"`
	Slack  slack          `toml:"slack"`
	Issues []Issue        `toml:"issue"`
	PR     PR             `toml:"pr"`
	Path   string         `toml:"path"` // path to plan

	broken bool `toml:"-"` // in-memory only: true if this plan couldn't be parsed
}

type slack struct {
	Title    string `toml:"title"`
	URL      string `toml:"url"`
	Body     string `toml:"body"`
	Waiting  bool   `toml:"waiting"`
	Resolved bool   `toml:"resolved"`
}

type Issue struct {
	Title  string `toml:"title"`
	URL    string `toml:"url"`
	Closed bool   `toml:"closed"`
}

type PR struct {
	Title     string    `toml:"title"`
	Mergeable string    `toml:"mergeable"`
	URL       string    `toml:"url"`
	Comments  []comment `toml:"comment"`
}

type comment struct {
	Title   string     `toml:"title"`
	Status  statusKind `toml:"status"`
	Source  string     `toml:"source"`
	Author  string     `toml:"author"`
	Thread  string     `toml:"thread"`
	FixRef  string     `toml:"fix_ref"`
	Comment string     `toml:"comment"`
	Plan    string     `toml:"plan"`
	Reply   string     `toml:"reply"`
}

func defaultPlan(title string) plan {
	due := time.Now().AddDate(0, 0, defaultDaysDue)
	return plan{
		Title:  title,
		Status: statusOpen,
		Due:    toml.LocalDate{Year: due.Year(), Month: int(due.Month()), Day: due.Day()},
	}
}

// localDateAsTime converts a toml.LocalDate to time.Time at midnight local time.
func localDateAsTime(d toml.LocalDate) time.Time {
	return time.Date(d.Year, time.Month(d.Month), d.Day, 0, 0, 0, 0, time.Local)
}

func readPlan(path string) (plan, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return plan{}, fmt.Errorf("read plan %q: %w", path, err)
	}
	var p plan
	if err := toml.Unmarshal(data, &p); err != nil {
		return plan{}, fmt.Errorf("parse plan %q: %w", path, err)
	}
	p.Path = path
	return p, nil
}

func writePlan(p plan) error {
	if p.Path == "" {
		return fmt.Errorf("write plan: empty path")
	}
	data, err := toml.Marshal(p)
	if err != nil {
		return fmt.Errorf("marshal plan: %w", err)
	}
	if err := os.WriteFile(p.Path, data, planFileMode); err != nil {
		return fmt.Errorf("write plan %q: %w", p.Path, err)
	}
	return nil
}

