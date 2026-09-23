package antigravity

import "testing"

func TestQuotaPools_NativeFirstThenByID(t *testing.T) {
	pools := QuotaPools(map[string]QuotaWindow{
		"zz-weekly":     {},
		"3p-5h":         {},
		"gemini-weekly": {},
		"gemini-5h":     {},
	})
	var ids []string
	for _, p := range pools {
		ids = append(ids, p.ID)
	}
	if len(ids) != 3 || ids[0] != NativePoolID || ids[1] != "3p" || ids[2] != "zz" {
		t.Fatalf("pool order=%v, want [gemini 3p zz]", ids)
	}
	if pools[0].Windows[0].Window != "5h" || pools[0].Windows[1].Window != "weekly" {
		t.Fatalf("native windows=%+v", pools[0].Windows)
	}
	// A pool the CLI adds later is kept under its raw id rather than dropped.
	if pools[2].Label != "zz" {
		t.Fatalf("unknown pool label=%q", pools[2].Label)
	}
}

func TestQuotaPools_ResetTime(t *testing.T) {
	pools := QuotaPools(map[string]QuotaWindow{
		"gemini-5h":     {ResetTime: "2026-09-23T09:00:38Z"},
		"gemini-weekly": {ResetTime: "not a time"},
	})
	if pools[0].Windows[0].ResetsAt.IsZero() {
		t.Fatal("RFC3339 reset_time must parse")
	}
	if !pools[0].Windows[1].ResetsAt.IsZero() {
		t.Fatal("unparseable reset_time must stay zero")
	}
}
