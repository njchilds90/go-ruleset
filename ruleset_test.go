package ruleset_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/njchilds90/go-ruleset"
)

// ─── Helpers ─────────────────────────────────────────────────────────────────

func mustEngine(rules ...ruleset.Rule) *ruleset.Engine {
	e := ruleset.New()
	for _, r := range rules {
		if err := e.AddRule(r); err != nil {
			panic(err)
		}
	}
	return e
}

func eval(t *testing.T, e *ruleset.Engine, facts ruleset.Facts) ruleset.EvalResult {
	t.Helper()
	res, err := e.Eval(context.Background(), facts)
	if err != nil {
		t.Fatalf("Eval returned unexpected error: %v", err)
	}
	return res
}

// ─── Operator Tests ──────────────────────────────────────────────────────────

func TestOpEq(t *testing.T) {
	e := mustEngine(ruleset.Rule{
		Name:       "eq-test",
		Conditions: []ruleset.Condition{{Fact: "status", Operator: ruleset.OpEq, Value: "active"}},
		Actions:    []ruleset.Action{{Type: "pass"}},
	})
	tests := []struct {
		facts  ruleset.Facts
		passed bool
	}{
		{ruleset.Facts{"status": "active"}, true},
		{ruleset.Facts{"status": "inactive"}, false},
		{ruleset.Facts{}, false},
	}
	for _, tt := range tests {
		res := eval(t, e, tt.facts)
		if got := len(res.PassedRules) > 0; got != tt.passed {
			t.Errorf("OpEq facts=%v: got passed=%v want %v", tt.facts, got, tt.passed)
		}
	}
}

func TestOpNeq(t *testing.T) {
	e := mustEngine(ruleset.Rule{
		Name:       "neq-test",
		Conditions: []ruleset.Condition{{Fact: "role", Operator: ruleset.OpNeq, Value: "guest"}},
		Actions:    []ruleset.Action{{Type: "pass"}},
	})
	tests := []struct {
		facts  ruleset.Facts
		passed bool
	}{
		{ruleset.Facts{"role": "admin"}, true},
		{ruleset.Facts{"role": "guest"}, false},
	}
	for _, tt := range tests {
		res := eval(t, e, tt.facts)
		if got := len(res.PassedRules) > 0; got != tt.passed {
			t.Errorf("OpNeq facts=%v: want passed=%v got %v", tt.facts, tt.passed, got)
		}
	}
}

func TestNumericOperators(t *testing.T) {
	tests := []struct {
		op     ruleset.Operator
		value  any
		fact   any
		passed bool
	}{
		{ruleset.OpLt, 18, 17, true},
		{ruleset.OpLt, 18, 18, false},
		{ruleset.OpLt, 18, 19, false},
		{ruleset.OpLte, 18, 18, true},
		{ruleset.OpLte, 18, 17, true},
		{ruleset.OpLte, 18, 19, false},
		{ruleset.OpGt, 18, 19, true},
		{ruleset.OpGt, 18, 18, false},
		{ruleset.OpGte, 18, 18, true},
		{ruleset.OpGte, 18, 17, false},
		// string numbers
		{ruleset.OpGte, "18", "21", true},
		// mixed
		{ruleset.OpGte, 18, "21", true},
	}
	for _, tt := range tests {
		e := mustEngine(ruleset.Rule{
			Name:       "num",
			Conditions: []ruleset.Condition{{Fact: "x", Operator: tt.op, Value: tt.value}},
			Actions:    []ruleset.Action{{Type: "ok"}},
		})
		res := eval(t, e, ruleset.Facts{"x": tt.fact})
		got := len(res.PassedRules) > 0
		if got != tt.passed {
			t.Errorf("op=%s value=%v fact=%v: got passed=%v want %v", tt.op, tt.value, tt.fact, got, tt.passed)
		}
	}
}

func TestOpContains(t *testing.T) {
	e := mustEngine(ruleset.Rule{
		Name:       "contains",
		Conditions: []ruleset.Condition{{Fact: "name", Operator: ruleset.OpContains, Value: "Go"}},
		Actions:    []ruleset.Action{{Type: "ok"}},
	})
	tests := []struct{ fact, passed any }{
		{"GoLang", true},
		{"Python", false},
	}
	for _, tt := range tests {
		res := eval(t, e, ruleset.Facts{"name": tt.fact})
		if got := len(res.PassedRules) > 0; got != tt.passed {
			t.Errorf("OpContains %v: got %v", tt.fact, got)
		}
	}
}

func TestOpIn(t *testing.T) {
	e := mustEngine(ruleset.Rule{
		Name:       "in-test",
		Conditions: []ruleset.Condition{{Fact: "tier", Operator: ruleset.OpIn, Value: []any{"gold", "platinum"}}},
		Actions:    []ruleset.Action{{Type: "ok"}},
	})
	tests := []struct {
		tier   string
		passed bool
	}{
		{"gold", true},
		{"platinum", true},
		{"silver", false},
	}
	for _, tt := range tests {
		res := eval(t, e, ruleset.Facts{"tier": tt.tier})
		if got := len(res.PassedRules) > 0; got != tt.passed {
			t.Errorf("OpIn tier=%s: got %v want %v", tt.tier, got, tt.passed)
		}
	}
}

func TestOpMatches(t *testing.T) {
	e := mustEngine(ruleset.Rule{
		Name:       "match",
		Conditions: []ruleset.Condition{{Fact: "email", Operator: ruleset.OpMatches, Value: `^[^@]+@[^@]+\.[^@]+$`}},
		Actions:    []ruleset.Action{{Type: "valid"}},
	})
	tests := []struct {
		email  string
		passed bool
	}{
		{"user@example.com", true},
		{"notanemail", false},
	}
	for _, tt := range tests {
		res := eval(t, e, ruleset.Facts{"email": tt.email})
		if got := len(res.PassedRules) > 0; got != tt.passed {
			t.Errorf("OpMatches email=%s: got %v want %v", tt.email, got, tt.passed)
		}
	}
}

func TestOpExistsNotExists(t *testing.T) {
	eExists := mustEngine(ruleset.Rule{
		Name:       "exists",
		Conditions: []ruleset.Condition{{Fact: "token", Operator: ruleset.OpExists}},
		Actions:    []ruleset.Action{{Type: "ok"}},
	})
	eNotExists := mustEngine(ruleset.Rule{
		Name:       "not-exists",
		Conditions: []ruleset.Condition{{Fact: "token", Operator: ruleset.OpNotExists}},
		Actions:    []ruleset.Action{{Type: "ok"}},
	})

	r1 := eval(t, eExists, ruleset.Facts{"token": "abc"})
	if len(r1.PassedRules) == 0 {
		t.Error("exists: expected pass when key present")
	}
	r2 := eval(t, eExists, ruleset.Facts{})
	if len(r2.PassedRules) > 0 {
		t.Error("exists: expected fail when key absent")
	}
	r3 := eval(t, eNotExists, ruleset.Facts{})
	if len(r3.PassedRules) == 0 {
		t.Error("not_exists: expected pass when key absent")
	}
}

// ─── Logic Tests ─────────────────────────────────────────────────────────────

func TestLogicAll(t *testing.T) {
	e := mustEngine(ruleset.Rule{
		Name:  "all",
		Logic: ruleset.LogicAll,
		Conditions: []ruleset.Condition{
			{Fact: "age", Operator: ruleset.OpGte, Value: 18},
			{Fact: "role", Operator: ruleset.OpEq, Value: "admin"},
		},
		Actions: []ruleset.Action{{Type: "grant"}},
	})
	tests := []struct {
		facts  ruleset.Facts
		passed bool
	}{
		{ruleset.Facts{"age": 25, "role": "admin"}, true},
		{ruleset.Facts{"age": 16, "role": "admin"}, false},
		{ruleset.Facts{"age": 25, "role": "user"}, false},
	}
	for _, tt := range tests {
		res := eval(t, e, tt.facts)
		got := len(res.PassedRules) > 0
		if got != tt.passed {
			t.Errorf("LogicAll facts=%v: got %v want %v", tt.facts, got, tt.passed)
		}
	}
}

func TestLogicAny(t *testing.T) {
	e := mustEngine(ruleset.Rule{
		Name:  "any",
		Logic: ruleset.LogicAny,
		Conditions: []ruleset.Condition{
			{Fact: "vip", Operator: ruleset.OpEq, Value: true},
			{Fact: "tier", Operator: ruleset.OpEq, Value: "platinum"},
		},
		Actions: []ruleset.Action{{Type: "allow"}},
	})
	tests := []struct {
		facts  ruleset.Facts
		passed bool
	}{
		{ruleset.Facts{"vip": true, "tier": "silver"}, true},
		{ruleset.Facts{"vip": false, "tier": "platinum"}, true},
		{ruleset.Facts{"vip": false, "tier": "silver"}, false},
	}
	for _, tt := range tests {
		res := eval(t, e, tt.facts)
		got := len(res.PassedRules) > 0
		if got != tt.passed {
			t.Errorf("LogicAny facts=%v: got %v want %v", tt.facts, got, tt.passed)
		}
	}
}

// ─── Dot Notation ────────────────────────────────────────────────────────────

func TestDotNotation(t *testing.T) {
	e := mustEngine(ruleset.Rule{
		Name:       "nested",
		Conditions: []ruleset.Condition{{Fact: "user.profile.age", Operator: ruleset.OpGte, Value: 18}},
		Actions:    []ruleset.Action{{Type: "ok"}},
	})
	facts := ruleset.Facts{
		"user": map[string]any{
			"profile": map[string]any{
				"age": 22,
			},
		},
	}
	res := eval(t, e, facts)
	if len(res.PassedRules) == 0 {
		t.Error("dot notation: expected pass for nested fact")
	}
}

// ─── Priority ────────────────────────────────────────────────────────────────

func TestPriority(t *testing.T) {
	e := ruleset.New()
	e.MustAddRule(ruleset.Rule{
		Name: "low", Priority: 1,
		Conditions: []ruleset.Condition{{Fact: "x", Operator: ruleset.OpEq, Value: "a"}},
		Actions:    []ruleset.Action{{Type: "low"}},
	})
	e.MustAddRule(ruleset.Rule{
		Name: "high", Priority: 10,
		Conditions: []ruleset.Condition{{Fact: "x", Operator: ruleset.OpEq, Value: "a"}},
		Actions:    []ruleset.Action{{Type: "high"}},
	})

	rr, ok, err := e.EvalFirst(context.Background(), ruleset.Facts{"x": "a"})
	if err != nil || !ok {
		t.Fatal("expected first match")
	}
	if rr.RuleName != "high" {
		t.Errorf("expected 'high' to fire first, got %q", rr.RuleName)
	}
}

// ─── Error Cases ─────────────────────────────────────────────────────────────

func TestDuplicateRuleName(t *testing.T) {
	e := ruleset.New()
	r := ruleset.Rule{Name: "dup", Conditions: []ruleset.Condition{{Fact: "x", Operator: ruleset.OpEq, Value: 1}}, Actions: []ruleset.Action{{Type: "ok"}}}
	if err := e.AddRule(r); err != nil {
		t.Fatal(err)
	}
	err := e.AddRule(r)
	var dupErr *ruleset.ErrDuplicateRule
	if !errors.As(err, &dupErr) {
		t.Errorf("expected ErrDuplicateRule, got %v", err)
	}
}

func TestEmptyRuleName(t *testing.T) {
	e := ruleset.New()
	err := e.AddRule(ruleset.Rule{Conditions: []ruleset.Condition{{Fact: "x", Operator: ruleset.OpEq, Value: 1}}})
	if !errors.Is(err, ruleset.ErrEmptyRuleName) {
		t.Errorf("expected ErrEmptyRuleName, got %v", err)
	}
}

func TestInvalidOperator(t *testing.T) {
	e := ruleset.New()
	err := e.AddRule(ruleset.Rule{
		Name:       "bad",
		Conditions: []ruleset.Condition{{Fact: "x", Operator: "unknown_op", Value: 1}},
		Actions:    []ruleset.Action{{Type: "ok"}},
	})
	var opErr *ruleset.ErrInvalidOperator
	if !errors.As(err, &opErr) {
		t.Errorf("expected ErrInvalidOperator, got %v", err)
	}
}

func TestContextCancellation(t *testing.T) {
	e := mustEngine(ruleset.Rule{
		Name:       "any",
		Conditions: []ruleset.Condition{{Fact: "x", Operator: ruleset.OpEq, Value: 1}},
		Actions:    []ruleset.Action{{Type: "ok"}},
	})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := e.Eval(ctx, ruleset.Facts{"x": 1})
	if !errors.Is(err, ruleset.ErrContextCanceled) {
		t.Errorf("expected ErrContextCanceled, got %v", err)
	}
}

// ─── RemoveRule ───────────────────────────────────────────────────────────────

func TestRemoveRule(t *testing.T) {
	e := mustEngine(ruleset.Rule{
		Name:       "to-remove",
		Conditions: []ruleset.Condition{{Fact: "x", Operator: ruleset.OpEq, Value: 1}},
		Actions:    []ruleset.Action{{Type: "ok"}},
	})
	if !e.RemoveRule("to-remove") {
		t.Fatal("expected RemoveRule to return true")
	}
	res := eval(t, e, ruleset.Facts{"x": 1})
	if len(res.PassedRules) > 0 {
		t.Error("expected no rules to fire after removal")
	}
}

// ─── JSON Serialization ──────────────────────────────────────────────────────

func TestEvalResultIsJSONSerializable(t *testing.T) {
	e := mustEngine(ruleset.Rule{
		Name:       "json-test",
		Conditions: []ruleset.Condition{{Fact: "score", Operator: ruleset.OpGte, Value: 80}},
		Actions:    []ruleset.Action{{Type: "pass", Params: map[string]any{"grade": "B+"}}},
	})
	res := eval(t, e, ruleset.Facts{"score": 90})
	b, err := json.Marshal(res)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}
	var back ruleset.EvalResult
	if err := json.Unmarshal(b, &back); err != nil {
		t.Fatalf("json.Unmarshal failed: %v", err)
	}
	if len(back.PassedRules) != 1 {
		t.Errorf("expected 1 passed rule after round-trip, got %d", len(back.PassedRules))
	}
}

// ─── ConditionResult Trace ───────────────────────────────────────────────────

func TestConditionResultTrace(t *testing.T) {
	e := mustEngine(ruleset.Rule{
		Name: "trace",
		Conditions: []ruleset.Condition{
			{Fact: "age", Operator: ruleset.OpGte, Value: 18},
			{Fact: "active", Operator: ruleset.OpEq, Value: true},
		},
		Actions: []ruleset.Action{{Type: "ok"}},
	})
	res := eval(t, e, ruleset.Facts{"age": 21, "active": false})
	if len(res.FailedRules) == 0 {
		t.Fatal("expected failed rule")
	}
	cr := res.FailedRules[0].ConditionResults
	if len(cr) != 2 {
		t.Fatalf("expected 2 condition results, got %d", len(cr))
	}
	if !cr[0].Passed {
		t.Error("age >= 18 should have passed")
	}
	if cr[1].Passed {
		t.Error("active == true should have failed (active=false)")
	}
}
