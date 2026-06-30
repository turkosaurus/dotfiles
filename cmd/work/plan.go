package main

import (
	"fmt"
	"os"
	"path"
	"time"

	"github.com/pelletier/go-toml/v2"
)

type StatusKind string

var (
	DefaultWorkDir  = path.Join(os.Getenv("HOME"), "ww")
	DefaultChoreDir = path.Join(DefaultWorkDir, "x")
	DefaultDaysDue  = 3
)

const (
	StatusOpen    StatusKind = "open"
	StatusPending StatusKind = "pending"
	StatusDone    StatusKind = "done"

	PlanFileMode os.FileMode = 0o644
)

type Plans struct {
	Plans []Plan `toml:"plan"`
}

type Plan struct {
	Title  string     `toml:"title"`
	Status StatusKind `toml:"status"`
	Due    time.Time  `toml:"due"`
	Tasks  []string   `toml:"tasks"`
	Slack  Slack      `toml:"slack"`
	Issue  Issue      `toml:"issue"`
	PR     PR         `toml:"pr"`
	Path   string     `toml:"path"` // path to plan
}

type Slack struct {
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
	Comments  []Comment `toml:"comment"`
}

type Comment struct {
	Title   string     `toml:"title"`
	Status  StatusKind `toml:"status"`
	Source  string     `toml:"source"`
	Author  string     `toml:"author"`
	Thread  string     `toml:"thread"`
	FixRef  string     `toml:"fix_ref"`
	Comment string     `toml:"comment"`
	Plan    string     `toml:"plan"`
	Reply   string     `toml:"reply"`
}

func NewDefaultPlan(title string) Plan {
	return Plan{
		Title:  title,
		Status: StatusOpen,
		Due:    time.Now().Add(time.Hour * 24 * time.Duration(DefaultDaysDue)), // FIXME: round to the nearest day
	}
}

func readPlan(path string) (Plan, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Plan{}, fmt.Errorf("read plan %q: %w", path, err)
	}
	var plan Plan
	if err := toml.Unmarshal(data, &plan); err != nil {
		return Plan{}, fmt.Errorf("parse plan %q: %w", path, err)
	}
	plan.Path = path
	return plan, nil
}

func writePlan(p Plan) error {
	if p.Path == "" {
		return fmt.Errorf("write plan: empty path")
	}
	data, err := toml.Marshal(p)
	if err != nil {
		return fmt.Errorf("marshal plan: %w", err)
	}
	if err := os.WriteFile(p.Path, data, PlanFileMode); err != nil {
		return fmt.Errorf("write plan %q: %w", p.Path, err)
	}
	return nil
}
