# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [1.0.0] - 2026-02-24

### Added
- `Engine` type for managing and evaluating rulesets
- `Rule` type with `Name`, `Priority`, `Logic`, `Conditions`, and `Actions` fields
- `Condition` type with dot-notation `Fact` path resolution
- 13 built-in operators: `eq`, `neq`, `lt`, `lte`, `gt`, `gte`, `contains`, `not_contains`, `in`, `not_in`, `matches`, `exists`, `not_exists`
- `LogicAll` (AND) and `LogicAny` (OR) rule logic modes
- `Eval` method: evaluate all rules, returns `EvalResult` with `PassedRules`, `FailedRules`, and flattened `Actions`
- `EvalFirst` method: return first passing rule in priority order
- `RemoveRule` method for dynamic rule management
- Per-condition evaluation trace via `ConditionResult`
- Full `context.Context` support with `ErrContextCanceled`
- Structured, typed error types: `ErrEmptyRuleName`, `ErrDuplicateRule`, `ErrInvalidOperator`, `ErrInvalidLogic`
- Priority-based rule ordering (higher priority evaluated first, stable sort)
- Nested fact resolution via dot notation (e.g. `"user.profile.age"`)
- Numeric type coercion for comparison operators (int, float64, string numbers)
- Full JSON serialization support for all types
- Zero external dependencies
- Table-driven test suite with race detector
- GitHub Actions CI across Go 1.21, 1.22, 1.23
- GoDoc examples for all exported functions
