# go-ruleset

[![CI](https://github.com/njchilds90/go-ruleset/actions/workflows/ci.yml/badge.svg)](https://github.com/njchilds90/go-ruleset/actions/workflows/ci.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/njchilds90/go-ruleset.svg)](https://pkg.go.dev/github.com/njchilds90/go-ruleset)
[![Go Report Card](https://goreportcard.com/badge/github.com/njchilds90/go-ruleset)](https://goreportcard.com/report/github.com/njchilds90/go-ruleset)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

**go-ruleset** is a zero-dependency, declarative rule/policy engine for Go.

Evaluate structured rulesets against arbitrary fact maps at runtime — ideal for feature flags, access control, pricing logic, data validation pipelines, and AI agent decision systems.
```
go get github.com/njchilds90/go-ruleset
```

---

## Why go-ruleset?

- ✅ **Zero external dependencies** — pure stdlib Go
- ✅ **Deterministic, traceable evaluation** — every condition produces a trace
- ✅ **JSON-serializable inputs and outputs** — machine-readable by design
- ✅ **Dot-notation fact paths** — `"user.profile.age"` resolves nested maps
- ✅ **context.Context support** — cancelation and deadlines respected
- ✅ **Structured, typed errors** — no string matching required
- ✅ **AI-agent friendly** — agents can generate, modify, and evaluate rules programmatically

---

## Quick Start
```go
package main

import (
    "context"
    "encoding/json"
    "fmt"

    "github.com/njchilds90/go-ruleset"
)

func main() {
    e := ruleset.New()

    e.MustAddRule(ruleset.Rule{
        Name:     "adult-admin-access",
        Priority: 10,
        Logic:    ruleset.LogicAll,
        Conditions: []ruleset.Condition{
            {Fact: "user.age", Operator: ruleset.OpGte, Value: 18},
            {Fact: "user.role", Operator: ruleset.OpIn, Value: []any{"admin", "superuser"}},
        },
        Actions: []ruleset.Action{
            {Type: "grant_access", Params: map[string]any{"level": "full"}},
        },
    })

    e.MustAddRule(ruleset.Rule{
        Name: "verified-email",
        Conditions: []ruleset.Condition{
            {Fact: "user.email", Operator: ruleset.OpMatches, Value: `^[^@]+@company\.com$`},
        },
        Actions: []ruleset.Action{
            {Type: "flag", Params: map[string]any{"badge": "corporate"}},
        },
    })

    facts := ruleset.Facts{
        "user": map[string]any{
            "age":   30,
            "role":  "admin",
            "email": "alice@company.com",
        },
    }

    result, err := e.Eval(context.Background(), facts)
    if err != nil {
        panic(err)
    }

    b, _ := json.MarshalIndent(result, "", "  ")
    fmt.Println(string(b))
}
```

---

## Operators

| Operator | Description |
|---|---|
| `eq` | Equality (type-coerced via string normalization) |
| `neq` | Inequality |
| `lt` | Less than (numeric) |
| `lte` | Less than or equal (numeric) |
| `gt` | Greater than (numeric) |
| `gte` | Greater than or equal (numeric) |
| `contains` | String contains substring, or slice contains value |
| `not_contains` | Inverse of contains |
| `in` | Fact value is within a provided list |
| `not_in` | Inverse of in |
| `matches` | Fact string matches a regular expression |
| `exists` | Fact key is present and non-nil |
| `not_exists` | Fact key is absent or nil |

---

## Logic Modes

| Logic | Behavior |
|---|---|
| `all` | All conditions must pass (AND). **Default.** |
| `any` | At least one condition must pass (OR). |

---

## Dot-Notation Fact Paths

Facts support arbitrarily nested `map[string]any` structures resolved via dot notation:
```go
facts := ruleset.Facts{
    "user": map[string]any{
        "profile": map[string]any{"age": 25},
    },
}
// Fact path: "user.profile.age" → 25
```

---

## Evaluation Modes

### `Eval` — evaluate all rules
```go
result, err := e.Eval(ctx, facts)
// result.PassedRules — all rules that fired
// result.FailedRules — all rules that did not fire
// result.Actions     — flattened actions from all passed rules
```

### `EvalFirst` — first passing rule (switch-style)
```go
rr, ok, err := e.EvalFirst(ctx, facts)
if ok {
    fmt.Println("Fired:", rr.RuleName)
    fmt.Println("Actions:", rr.Actions)
}
```

---

## Condition Trace

Every `RuleResult` includes a per-condition trace for debugging and AI agent introspection:
```json
{
  "rule_name": "adult-admin-access",
  "passed": false,
  "condition_results": [
    {"fact": "user.age", "operator": "gte", "expected_value": 18, "actual_value": 15, "passed": false},
    {"fact": "user.role", "operator": "in", "expected_value": ["admin","superuser"], "actual_value": "admin", "passed": true}
  ]
}
```

---

## Error Types
```go
var ErrEmptyRuleName   = errors.New("go-ruleset: rule name must not be empty")
var ErrContextCanceled = errors.New("go-ruleset: evaluation canceled by context")

type ErrDuplicateRule    struct{ Name     string }
type ErrInvalidOperator  struct{ Operator Operator }
type ErrInvalidLogic     struct{ Logic    Logic }
```

---

## AI Agent Integration

go-ruleset is designed for use in AI agent pipelines:

1. **Generate rules as JSON** → unmarshal into `[]Rule`
2. **Evaluate against structured data** → `Eval(ctx, facts)`
3. **Inspect traces** → `ConditionResults` show exactly why rules fired or didn't
4. **Route on actions** → `result.Actions` is a typed, serializable action list

---

## Versioning

This project follows [Semantic Versioning](https://semver.org/).  
Current stable release: **v1.0.0**

---

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md).

## License

MIT — see [LICENSE](LICENSE).
