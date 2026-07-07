# docs/ — Agent Documentation Index

This directory contains reference documents for AI agents working on this project. Each file answers a specific class of questions.

| File | Use this when the agent needs to... |
|---|---|
| [ARCHITECTURE.md](ARCHITECTURE.md) | Understand system architecture, request flow, middleware chain, tech stack, deployment model |
| [STANDARDS.md](STANDARDS.md) | Learn coding conventions, Clean Architecture rules, FHIR client patterns, error handling, testing approach |
| [KNOWN-PITFALLS.md](KNOWN-PITFALLS.md) | Avoid common mistakes: double-slash FHIR URLs, legacy deps, rate limiting limits, config gotchas |
| [STRUCTURE.md](STRUCTURE.md) | Navigate the repository: what each directory contains, which files to modify for a given task |
| [DEEPSOURCE-FIX-PATTERNS.md](DEEPSOURCE-FIX-PATTERNS.md) | Resolve common DeepSource static analysis issues: unused params/receivers, log.Fatalf, complexity, dead code, test helpers |

## How to Use

1. **Start** at root [AGENTS.md](../AGENTS.md) for project orientation
2. **Navigate** to the relevant subdirectory AGENTS.md (`cmd/`, `internal/app/`, `internal/pkg/`)
3. **Consult** docs/ files for depth — each answers a specific question
4. **Update** `KNOWN-PITFALLS.md` when discovering new traps during implementation

## Contribution Rules

- New docs go in `docs/` with descriptive filenames in `UPPER-CASE.md`
- Each doc ≤ 100 lines, focused on a single concern
- Update `AGENTS.md` (this file) when adding or changing a doc
