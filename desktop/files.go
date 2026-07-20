package desktop

import (
	"context"
	"fmt"
	"notsofast/core"
	"strings"
)

func (a *App) filesPage(ctx context.Context, query string, offset int) (any, error) {
	if a.service == nil {
		return nil, fmt.Errorf("connect a repository first")
	}
	if offset < 0 || offset > 100000 || len(query) > 4096 {
		return nil, fmt.Errorf("invalid file query")
	}
	head, err := a.service.Head(ctx, "local", "workspace")
	if err != nil {
		return nil, err
	}
	snapshot, err := a.service.Snapshot(ctx, "local", "workspace", head)
	if err != nil {
		return nil, err
	}
	matching := []core.Entry{}
	for _, entry := range snapshot.Entries {
		if strings.Contains(strings.ToLower(entry.Path), strings.ToLower(query)) {
			matching = append(matching, entry)
		}
	}
	total := len(matching)
	if offset > total {
		offset = total
	}
	end := offset + 100
	if end > total {
		end = total
	}
	return map[string]any{"entries": matching[offset:end], "total": total, "offset": offset, "snapshot": head, "has_more": end < total}, nil
}
