package desktop

import (
	"encoding/json"
	"fmt"
)

// Profile contains portable preferences and a draft, never evidence or credentials.
type Profile struct {
	Name      string   `json:"name"`
	Mode      string   `json:"mode"`
	Scheme    string   `json:"scheme"`
	Motion    bool     `json:"motion"`
	DraftPath string   `json:"draft_path"`
	Draft     string   `json:"draft"`
	Bookmarks []string `json:"bookmarks"`
}

func (p Profile) validate() error {
	if len(p.Name) > 100 || len(p.DraftPath) > 1024 || len(p.Draft) > 65536 || len(p.Bookmarks) > 50 {
		return fmt.Errorf("profile or draft is too large")
	}
	switch p.Mode {
	case "system", "light", "dark":
	default:
		return fmt.Errorf("select a supported brightness")
	}
	switch p.Scheme {
	case "lime", "ocean", "violet", "rose", "amber":
	default:
		return fmt.Errorf("select a supported color scheme")
	}

	for _, bookmark := range p.Bookmarks {
		if _, _, err := parseGitHubURL(bookmark); err != nil {
			return err
		}
	}
	return nil
}
func (a *App) profile() Profile {
	p := Profile{Name: "Local operator", Mode: "system", Scheme: "lime", Bookmarks: []string{}}
	var raw string
	if a.db.QueryRow("SELECT value FROM settings WHERE key='profile'").Scan(&raw) == nil {
		_ = json.Unmarshal([]byte(raw), &p)
	}
	return p
}
func (a *App) saveProfile(p Profile) error {
	if err := p.validate(); err != nil {
		return err
	}
	b, _ := json.Marshal(p)
	_, err := a.db.Exec("INSERT OR REPLACE INTO settings(key,value) VALUES('profile',?)", string(b))
	return err
}
func (a *App) insights() (any, error) {
	var workspaces, actions, published, pending, models int
	for _, q := range []struct {
		sql string
		out *int
	}{
		{"SELECT count(*) FROM workspaces", &workspaces},
		{"SELECT count(*) FROM events", &actions},
		{"SELECT count(*) FROM (SELECT DISTINCT source,json_extract(result,'$.id') FROM events WHERE json_extract(result,'$.outcome')='PUBLISHED')", &published},
		{"SELECT count(*) FROM events WHERE state='PENDING'", &pending},
		{"SELECT count(*) FROM model_runs", &models},
	} {
		if err := a.db.QueryRow(q.sql).Scan(q.out); err != nil {
			return nil, err
		}
	}
	return map[string]int{"workspaces": workspaces, "actions": actions, "published": published, "pending": pending, "model_trials": models}, nil
}
