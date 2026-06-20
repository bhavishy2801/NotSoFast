package protocol

import (
	"context"
	"encoding/json"
	"notsofast/core"
	"testing"
)

func TestStrictDispatch(t *testing.T) {
	s, e := core.Open(core.Config{Root: t.TempDir()})
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	for _, tc := range []struct{ op, body, code string }{{"search", `{"shell":"bad"}`, "BAD_REQUEST"}, {"shell", `{}`, "BAD_REQUEST"}, {"head", `{"repository":"private"}`, "FORBIDDEN"}, {"head", `{} {}`, "BAD_REQUEST"}} {
		_, e = Dispatch(context.Background(), s, "nobody", tc.op, json.RawMessage(tc.body), false)
		if core.Code(e) != tc.code {
			t.Fatalf("%s: %v", tc.op, e)
		}
	}
}
