# DEEPSOURCE-FIX-PATTERNS.md — Common Fix Patterns for Static Analysis Issues

This document catalogs repeatable patterns for resolving Go static analysis issues from DeepSource (or similar linters). Use this when fixing analogous issues.

## Unused Parameters

```
func doThing(ctx context.Context, unused string) { ... }
```

Use `_` as the parameter name. The call site keeps passing the original argument — Go discards it at the callee.

```
func doThing(ctx context.Context, _ string) { ... }
```

**Apply when**: the parameter is unused and cannot be removed (interface compliance, callback signature, or the value is needed by callers).

## Unused Receivers

```
func (s *Service) doThing() { ... }
// s is never used in the body
```

Remove the receiver and make it a standalone function. Update all call sites from `s.doThing()` to `doThing()`.

```
func doThing() { ... }
```

**Apply when**: the value receiver or pointer receiver is unused in the entire method body. Do NOT remove if the method is on an interface — Go requires receiver matching.

## Dead Code (unused types, functions, files)

```
type User struct { ... }
func parseUser(r *http.Request) *User { ... }
```

If both the type and all its consumers are unreferenced, remove the entire file (or the isolated block).

**Check**: search `git grep` across the module to confirm zero references. Watch for:
- Exported symbols referenced in other packages
- Interface implementations
- `init()` functions

## log.Fatalf in Non-Main Functions

```
func validate(cfg *Config) {
    if cfg.Key == "" {
        log.Fatalf("key required")
    }
}
```

Return `error` instead. The top-level caller (typically `init()` or `main()`) calls `log.Fatal` once:

```
func validate(cfg *Config) error {
    if cfg.Key == "" {
        return fmt.Errorf("key required in %s environment", cfg.Env)
    }
    return nil
}
```

**Apply when**: the function is called from `init()` or other startup code where `log.Fatal` is acceptable at the call site but not inside reusable validation logic.

## Cyclomatic Complexity

Extract branches and loops into a helper method with a descriptive name. The extracted helper is independently testable and reduces the parent's complexity.

**Check**: count `if`, `for`, `case`, `&&`, `||` in the function body. Below 10 is usually safe. Above 15 warrants extraction.

## os.Setenv → testing.Setenv

```
func setupEnv(k, v string) func() {
    orig := os.Getenv(k)
    os.Setenv(k, v)
    return func() { os.Setenv(k, orig) }
}
```

Replace with `t.Setenv(k, v)` available since Go 1.17. It auto-restores on test end and panics if the test runs in parallel (safety).

```
t.Setenv("MY_KEY", "my_value")
```

**Apply when**: the test uses `os.Setenv` to set environment variables. Remove the old `setupEnv`/`unsetEnv` helpers and all `defer cleanup()` calls.
