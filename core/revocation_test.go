package core

import (
	"context"
	"testing"
)

func TestRemovedRepositoryRevokesEpoch(t *testing.T) {
	s, _, _ := fixture(t)
	ctx := context.Background()
	dir := s.repo("demo")
	old, e := s.policyEpoch(ctx, "demo")
	if e != nil {
		t.Fatal(e)
	}
	cfg := s.cfg
	s.Close()
	delete(cfg.Repositories, "demo")
	next, e := Open(cfg)
	if e != nil {
		t.Fatal(e)
	}
	defer next.Close()
	epoch, e := gitText(ctx, dir, nil, "rev-parse", policyRef)
	if e != nil || epoch == old {
		t.Fatal("removed repository retained authorization epoch", epoch, e)
	}
}
func TestRegistrationRepairsMissingEpoch(t *testing.T) {
	s, _, head := fixture(t)
	ctx := context.Background()
	if _, e := git(ctx, s.repo("demo"), nil, "update-ref", "-d", policyRef); e != nil {
		t.Fatal(e)
	}
	if _, e := s.Register(ctx, "alice", "demo", head); e != nil {
		t.Fatal(e)
	}
	if _, e := s.policyEpoch(ctx, "demo"); e != nil {
		t.Fatal(e)
	}
}
