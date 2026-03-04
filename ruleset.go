// Package ruleset provides a zero-dependency, declarative rule/policy engine for Go.
// Rules are defined as structured data and evaluated against arbitrary fact maps,
// making them ideal for runtime policy evaluation, feature flags, access control,
// pricing logic, and AI agent decision pipelines.
//
// Key design principles:
//   - Zero external dependencies
//   - Deterministic, inspectable evaluation
//   - Machine-readable inputs and outputs
//   - context.Context support for deadline/cancellation
//   - Structured, typed errors
//   - Pure functions, no hidden global state
package ruleset

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"sync"
)

// ─── Operator ────────────────────────────────────────────────────────────────

// Operator defines the comparison operation applied between a fact value and a condition value.
type Operator string

const (
	// OpEq checks equality (==).
	OpEq Operator = "eq"
	// OpNeq checks inequality (!=).
	OpNeq Operator = "neq"
	// OpLt checks less than (<). Numeric comparison.
	OpLt Operator = "lt"
	// OpLte checks less than or equal (<=). Numeric comparison.
	OpLte Operator = "lte"
	// OpGt checks greater than (>). Numeric comparison.
	OpGt Operator = "gt"
	// OpGte checks greater than or equal (>=). Numeric comparison.
	OpGte Operator = "gte"
	// OpContains checks if a string fact contains a substring, or if a slice contains a value.
	OpContains Operator = "contains"
	// OpNotContains is the inverse of OpContains.
	OpNotContains Operator = "not_contains"
	// OpIn checks if a fact value is within a provided slice of values.
	OpIn Operator = "in"
	// OpNotIn is the inverse of OpIn.
	OpNotIn Operator = "not_in"
	// OpMatches checks if a string fact matches a regular expression pattern.
	OpMatches Operator = "matches"
	// OpExists checks if a fact key is present (non-nil) in the facts map.
	OpExists Operator = "exists"
	// OpNotExists checks if a fact key is absent or nil in the facts map.
	OpNotExists Operator = "not_exists"
)

// ─── Logic ───────────────────────────────────────────────────────────────────

// Logic defines how multiple conditions within a Rule are combined.
type Logic string

const (
	// LogicAll requires all conditions to pass (AND logic). Default.
	LogicAll Logic = "all"
	// LogicAny requires at least one condition to pass (OR logic).
	LogicAny Logic = "any"
)

// ─── Priority ────────────────────────────────────────────────────────────────

// Priority is an integer that controls rule evaluation order. Higher values run first.
// Rules with equal priority run in the order they were added.
type Priority int

// ─── Condition ───────────────────────────────────────────────────────────────

// Condition is a single predicate evaluated against the facts map.
// It compares the value at Fact (a dot-notation key path) against Value using Operator.
//
// Example:
//
//	Condition{Fact: "user.age", Operator: OpGte, Value: 18}
type Condition struct {
	// Fact is the key path into the facts map. Supports dot notation: "user.role".
	Fact string `json:"fact"` 
	// Operator is the comparison to apply.
	Operator Operator `json:"operator"` 
	// Value is the right-hand side of the comparison.
	// For OpExists/OpNotExists, Value is ignored.
	// For OpIn/OpNotIn, Value must be a []any or []string or []float64.
	Value any `json:"value"` 
}

// ─── Action ──────────────────────────────────────────────────────────────────

// Action is a structured result attached to a Rule that fires when conditions pass.
// It carries a Type tag and arbitrary Params for downstream consumers.
//
// Example:
//
//	Action{Type: "grant_access", Params: map[string]any{"role": "admin"}}
type Action struct {
	// Type is a machine-readable label for what should happen.
	Type string `json:"type"` 
	// Params carries structured data relevant to the action.
	Params map[string]any `json:"params,omitempty"` 
}

// ─── Rule ────────────────────────────────────────────────────────────────────

// Rule defines a named policy: when Conditions pass (according to Logic), Actions fire.
//
// Example:
//
//	Rule{
//	    Name:       "adult-access",
//	    Priority:   10,
//	    Logic:      LogicAll,
//	    Conditions: []Condition{{Fact: "user.age", Operator: OpGte, Value: 18}},
//	    Actions:    []Action{{Type: "allow"}},
//	}
type Rule struct {
	// Name is a human-readable identifier for the rule.
	Name string `json:"name"` 
	// Priority controls evaluation order. Higher = earlier. Default 0.
	Priority Priority `json:"priority,omitempty"` 
	// Logic determines how Conditions are combined. Defaults to LogicAll.
	Logic Logic `json:"logic,omitempty"` 
	// Conditions is the set of predicates to evaluate.
	Conditions []Condition `json:"conditions"` 
	// Actions is the set of structured outcomes produced when conditions pass.
	Actions []Action `json:"actions"` 
}

// ─── RuleResult ──────────────────────────────────────────────────────────────

// RuleResult is the outcome of evaluating a single Rule.
type RuleResult struct {
	// RuleName is the name of the evaluated rule.
	RuleName string `json:"rule_name"` 
	// Passed is true if the rule's conditions were satisfied.
	Passed bool `json:"passed"` 
	// Actions contains the rule's actions, populated only when Passed is true.
	Actions []Action `json:"actions,omitempty"` 
	// ConditionResults is a per-condition trace, useful for debugging and AI agents.
	ConditionResults []ConditionResult `json:"condition_results"` 
}

// ConditionResult is the per-condition evaluation trace within a RuleResult.
type ConditionResult struct {
	// Fact is the key path that was looked up.
	Fact string `json:"fact"` 
	// Operator is the operator that was applied.
	Operator Operator `json:"operator"` 
	// ExpectedValue is the right-hand side of the condition.
	ExpectedValue any `json:"expected_value"` 
	// ActualValue is what was found in the facts map.
	ActualValue any `json:"actual_value"` 
	// Passed is true if this individual condition was satisfied.
	Passed bool `json:"passed"` 
}

// EvalResult is the aggregate result of evaluating all rules in an Engine against a facts map.
type EvalResult struct {
	// PassedRules contains all rules whose conditions were satisfied.
	PassedRules []RuleResult `json:"passed_rules"` 
	// FailedRules contains all rules whose conditions were not satisfied.
	FailedRules []RuleResult `json:"failed_rules"` 
	// Actions is the flattened union of all actions from PassedRules, in priority order.
	Actions []Action `json:"actions"` 
}

// ─── Errors ──────────────────────────────────────────────────────────────────

// ErrInvalidOperator is returned when an unrecognized operator is used in a Condition.
type ErrInvalidOperator struct {
	Operator Operator
}

func (e *ErrInvalidOperator) Error() string {
	return fmt.Sprintf("go-ruleset: invalid operator %q", e.Operator)
}

// ErrInvalidLogic is returned when an unrecognized logic value is used in a Rule.
type ErrInvalidLogic struct {
	Logic Logic
}

func (e *ErrInvalidLogic) Error() string {
	return fmt.Sprintf("go-ruleset: invalid logic %q", e.Logic)
}

// ErrEmptyRuleName is returned when a Rule is added with an empty name.
var ErrEmptyRuleName = errors.New("go-ruleset: rule name must not be empty")

// ErrDuplicateRule is returned when a Rule with the same name is added twice.
type ErrDuplicateRule struct {
	Name string
}

func (e *ErrDuplicateRule) Error() string {
	return fmt.Sprintf("go-ruleset: duplicate rule name %q", e.Name)
}

// ErrContextCanceled is returned when evaluation is halted due to context cancellation.
var ErrContextCanceled = errors.New("go-ruleset: evaluation canceled by context")

// ─── Engine ──────────────────────────────────────────────────────────────────

// Engine holds a collection of Rules and evaluates them against a facts map.
// Engine is safe for concurrent access.
//
// Example:
//
//	e := ruleset.New()
//	e.AddRule(ruleset.Rule{
//	    Name:       "is-adult",
//	    Conditions: []ruleset.Condition{{Fact: "age", Operator: ruleset.OpGte, Value: 18}},
//	    Actions:    []ruleset.Action{{Type: "allow"}},
//	})
//	result, _ := e.Eval(context.Background(), ruleset.Facts{"age": 21})
type Engine struct {
	mu    sync.RWMutex
	rules []Rule
	names map[string]struct{}
}

// New creates a new, empty Engine.
//
// Example:
//
//	e := ruleset.New()
func New() *Engine {
	return &Engine{names: make(map[string]struct{})}
}

// AddRule adds a Rule to the Engine. Returns an error if the rule name is empty
// or a rule with the same name already exists.
//
// Example:
//
//	err := e.AddRule(ruleset.Rule{
//	    Name:       "flag-minor",
//	    Conditions: []ruleset.Condition{{Fact: "age", Operator: ruleset.OpLt, Value: 18}},
//	    Actions:    []ruleset.Action{{Type: "flag", Params: map[string]any{"reason": "minor"}}},
//	})
func (e *Engine) AddRule(r Rule) error {
	if r.Name == "" {
		return ErrEmptyRuleName
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	if _, exists := e.names[r.Name]; exists {
		return &ErrDuplicateRule{Name: r.Name}
	}
	if r.Logic == "" {
		r.Logic = LogicAll
	}
	if err := validateRule(r); err != nil {
		return err
	}
	e.rules = append(e.rules, r)
	e.names[r.Name] = struct{}{}
	sortRules(e.rules)
	return nil
}

// MustAddRule is like AddRule but panics on error. Useful for package-level initialization.
//
// Example:
//
//	e.MustAddRule(ruleset.Rule{Name: "deny-all", Conditions: []ruleset.Condition{{Fact: "role", Operator: ruleset.OpEq, Value: "blocked"}}, Actions: []ruleset.Action{{Type: "deny"}}})
func (e *Engine) MustAddRule(r Rule) {
	if err := e.AddRule(r); err != nil {
		panic(err)
	}
}

// Rules returns a copy of the Engine's rules in evaluation order (highest priority first).
func (e *Engine) Rules() []Rule {
	e.mu.RLock()
	defer e.mu.RUnlock()
	cp := make([]Rule, len(e.rules))
	copy(cp, e.rules)
	return cp
}

// RemoveRule removes a rule by name. Returns false if not found.
func (e *Engine) RemoveRule(name string) bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	for i, r := range e.rules {
		if r.Name == name {
			e.rules = append(e.rules[:i], e.rules[i+1:]...)
			delete(e.names, name)
			return true
		}
	}
	return false
}

// Facts is the input map evaluated against rules. Keys support dot-notation for nested access.
// Values may be any comparable Go type: string, float64, bool, int, []any, map[string]any.
//
// Example:
//
//	facts := ruleset.Facts{
//	    "user": map[string]any{"age": 25, "role": "admin"},
//	    "plan": "pro",
//	}
type Facts map[string]any

// Eval evaluates all rules in the Engine against the provided facts, returning an EvalResult.
// Evaluation is performed in priority order (highest priority first).
// If ctx is canceled before evaluation completes, ErrContextCanceled is returned.
//
// Example:
//
//	result, err := e.Eval(context.Background(), ruleset.Facts{"age": 30, "role": "admin"})
//	if err != nil {
//	    log.Fatal(err)
//	}
//	fmt.Println(result.Actions)
func (e *Engine) Eval(ctx context.Context, facts Facts) (EvalResult, error) {
	var res EvalResult
	e.mu.RLock()
	rules := make([]Rule, len(e.rules))
	copy(rules, e.rules)
	e.mu.RUnlock()

	for _, rule := range rules {
		select {
		case <-ctx.Done():
			return EvalResult{}, ErrContextCanceled
		default: 
		}
		rr, err := evalRule(rule, facts)
		if err != nil {
			return EvalResult{}, err
		}
		if rr.Passed {
			res.PassedRules = append(res.PassedRules, rr)
			res.Actions = append(res.Actions, rr.Actions...)
		} else {
			res.FailedRules = append(res.FailedRules, rr)
		}
	}
	return res, nil
}

// EvalFirst evaluates rules in priority order and returns the first passing RuleResult.
// If no rules pass, ok is false. Useful for exclusive-match / switch-style logic.
//
// Example:
//
//	rr, ok, err := e.EvalFirst(context.Background(), ruleset.Facts{"tier": "gold"})
func (e *Engine) EvalFirst(ctx context.Context, facts Facts) (RuleResult, bool, error) {
	e.mu.RLock()
	rules := make([]Rule, len(e.rules))
	copy(rules, e.rules)
	e.mu.RUnlock()

	for _, rule := range rules {
		select {
		case <-ctx.Done():
			return RuleResult{}, false, ErrContextCanceled
		default: 
		}
		rr, err := evalRule(rule, facts)
		if err != nil {
			return RuleResult{}, false, err
		}
		if rr.Passed {
			return rr, true, nil
		}
	}
	return RuleResult{}, false, nil
}

// ─── Internal evaluation ─────────────────────────────────────────────────────

func evalRule(rule Rule, facts Facts) (RuleResult, error) {
	rr := RuleResult{RuleName: rule.Name}
	condResults := make([]ConditionResult, 0, len(rule.Conditions))

	for _, cond := range rule.Conditions {
		actual := lookupFact(facts, cond.Fact)
		passed, err := evalCondition(cond.Operator, actual, cond.Value)
		if err != nil {
			return RuleResult{}, err
		}
		condResults = append(condResults, ConditionResult{
			Fact:          cond.Fact,
			Operator:      cond.Operator,
			ExpectedValue: cond.Value,
			ActualValue:   actual,
			Passed:        passed,
		})
	}
	rr.ConditionResults = condResults

	switch rule.Logic {
	case LogicAll, "":
		rr.Passed = allPassed(condResults)
	case LogicAny:
		rr.Passed = anyPassed(condResults)
	default: 
		return RuleResult{}, &ErrInvalidLogic{Logic: rule.Logic}
	}

	if rr.Passed {
		rr.Actions = rule.Actions
	}
	return rr, nil
}

func evalCondition(op Operator, actual, expected any) (bool, error) {
	switch op {
	case OpExists:
		return actual != nil, nil
	case OpNotExists:
		return actual == nil, nil
	case OpEq:
		return toStringNorm(actual) == toStringNorm(expected), nil
	case OpNeq:
		return toStringNorm(actual) != toStringNorm(expected), nil
	case OpLt, OpLte, OpGt, OpGte:
		return numericCompare(op, actual, expected)
	case OpContains:
		return containsCheck(actual, expected)
	case OpNotContains:
		ok, err := containsCheck(actual, expected)
		return !ok, err
	case OpIn:
		return inCheck(actual, expected)
	case OpNotIn:
		ok, err := inCheck(actual, expected)
		return !ok, err
	case OpMatches:
		return matchesCheck(actual, expected)
	default: 
		return false, &ErrInvalidOperator{Operator: op}
	}
}

// lookupFact resolves a dot-notation path like "user.profile.age" from a facts map.
func lookupFact(facts Facts, path string) any {
	parts := strings.Split(path, ".")
	var cur any = map[string]any(facts)
	for _, part := range parts {
		switch m := cur.(type) {
		case map[string]any:
			v, ok := m[part]
			if !ok {
				return nil
			}
			cur = v
		case Facts:
			v, ok := m[part]
			if !ok {
				return nil
			}
			cur = v
		default: 
			rv := reflect.ValueOf(cur)
			if rv.Kind() == reflect.Map {
				kv := reflect.ValueOf(part)
				if !kv.Type().AssignableTo(rv.Type().Key()) {
					return nil
				}
				v := rv.MapIndex(kv)
				if !v.IsValid() {
					return nil
				}
				cur = v.Interface()
			} else {
				return nil
			}
		}
	}
	return cur
}

func toFloat(v any) (float64, bool) {
	switch x := v.(type) {
	case float64:
		return x, true
	case float32:
		return float64(x), true
	case int:
		return float64(x), true
	case int8:
		return float64(x), true
	case int16:
		return float64(x), true
	case int32:
		return float64(x), true
	case int64:
		return float64(x), true
	case uint:
		return float64(x), true
	case uint8:
		return float64(x), true
	case uint16:
		return float64(x), true
	case uint32:
		return float64(x), true
	case uint64:
		return float64(x), true
	case string:
		f, err := strconv.ParseFloat(x, 64)
		if err != nil {
			return 0, false
		}
		return f, true
	}
	return 0, false
}

func numericCompare(op Operator, actual, expected any) (bool, error) {
	a, aok := toFloat(actual)
	b, bok := toFloat(expected)
	if !aok || !bok {
		return false, nil
	}
	switch op {
	case OpLt:
		return a < b, nil
	case OpLte:
		return a <= b, nil
	case OpGt:
		return a > b, nil
	case OpGte:
		return a >= b, nil
	}
	return false, nil
}

func toStringNorm(v any) string {
	if v == nil {
		return ""
	}
	if f, ok := toFloat(v); ok {
		// Normalize numbers so "18" == 18 == 18.0
		if f == float64(int64(f)) {
			return strconv.FormatInt(int64(f), 10)
		}
		return strconv.FormatFloat(f, 'f', -1, 64)
	}
	return fmt.Sprintf("%v", v)
}

func containsCheck(actual, expected any) (bool, error) {
	// String contains string
	as, aIsStr := actual.(string)
	es, eIsStr := expected.(string)
	if aIsStr && eIsStr {
		return strings.Contains(as, es), nil
	}
	// Slice contains value
	rv := reflect.ValueOf(actual)
	if rv.Kind() == reflect.Slice {
		es2 := toStringNorm(expected)
		for i := 0; i < rv.Len(); i++ {
			if toStringNorm(rv.Index(i).Interface()) == es2 {
				return true, nil
			}
		}
		return false, nil
	}
	return false, nil
}

func inCheck(actual, expected any) (bool, error) {
	rv := reflect.ValueOf(expected)
	if rv.Kind() != reflect.Slice {
		return false, nil
	}
	as := toStringNorm(actual)
	for i := 0; i < rv.Len(); i++ {
		if toStringNorm(rv.Index(i).Interface()) == as {
			return true, nil
		}
	}
	return false, nil
}

func matchesCheck(actual, expected any) (bool, error) {
	as, ok := actual.(string)
	if !ok {
		as = fmt.Sprintf("%v", actual)
	}
	pattern, ok := expected.(string)
	if !ok {
		return false, nil
	}
	re, err := regexp.Compile(pattern)
	if err != nil {
		return false, fmt.Errorf("go-ruleset: invalid regex pattern %q: %w", pattern, err)
	}
	return re.MatchString(as), nil
}

func allPassed(results []ConditionResult) bool {
	for _, r := range results {
		if !r.Passed {
			return false
		}
	}
	return true
}

func anyPassed(results []ConditionResult) bool {
	for _, r := range results {
		if r.Passed {
			return true
		}
	}
	return false
}

func validateRule(r Rule) error {
	if r.Logic != "" && r.Logic != LogicAll && r.Logic != LogicAny {
		return &ErrInvalidLogic{Logic: r.Logic}
	}
	for _, c := range r.Conditions {
		switch c.Operator {
		case OpEq, OpNeq, OpLt, OpLte, OpGt, OpGte, OpContains, OpNotContains,
			OpIn, OpNotIn, OpMatches, OpExists, OpNotExists:
			// valid
		default: 
			return &ErrInvalidOperator{Operator: c.Operator}
		}
	}
	return nil
}

// sortRules sorts by Priority descending (stable, preserves insertion order for ties).
func sortRules(rules []Rule) {
	// insertion sort (small N, stable)
	for i := 1; i < len(rules); i++ {
		for j := i; j > 0 && rules[j].Priority > rules[j-1].Priority; j-- {
			rules[j], rules[j-1] = rules[j-1], rules[j]
		}
	}
}