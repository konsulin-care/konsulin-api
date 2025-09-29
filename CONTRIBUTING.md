# Contributing Guide

Thanks for helping improve this project! This document explains how we work, how to create a high-quality Pull Request (PR), how to file issues (we require using an LLM prompt; see below), and what we expect from contributors.

> TL;DR
> 1. Fork the repository.
> 2. Create a focused branch from `develop` (e.g. `feature/123-short-desc`).
> 3. Commit changes on your fork/branch.
> 4. Open a Pull Request against `develop`.
> 5. Fill the PR description using the PR template below and include testing evidence.

## 1. Quick workflow (required)

1. **Fork** the repository to your account.
2. **Pull** latest `develop` from upstream and **create a branch** from it:
   - Branch naming examples:
     - `feature/123-short-desc`
     - `fix/456-short-desc`
     - `chore/docs-update-789`
     - `hotfix/urgent-123`
3. Make **small, focused commits**. Use clear, imperative commit messages (see Commit style).
4. Run linting and tests locally (see next sections / project README).
5. Push your branch to your fork.
6. Open a **Pull Request** into `upstream/main` and use the PR template below.
7. Ask for reviewers and respond to feedback. Keep discussion on the PR.
8. After approvals and passing CI, a maintainer will merge (see Merge policy).

> **Note:** Always start work from the latest `develop`. Rebase/merge `develop` into your branch when asked.

## 2. Commit message style (recommended)

- Use an imperative, concise subject line (<= 50 characters).
- Optionally include a longer body with motivation and context (wrapped at ~72 chars).
- Reference related issues: `Fixes #123` or `Related #456`.

**Example**
```
Add subject mapping for ServiceRequest (FHIR)

- Use `Patient/<id>` when target is patient
- Use `Group/<role>` for role-based services
- Ensure required groups exist on startup

Fixes #234
```

## 3. Pull Request

All contributions must come via PR. PRs are the place to explain intent, show evidence, and make reviews fast.

### Required PR metadata
- Link to the related issue (if any).
- A clear **Summary** one-liner.
- **Purpose / Motivation**: why this change is needed.
- **How to test**: exact steps to validate locally and any commands.
- **Evidence**: logs, API responses, screenshots, test output, or CI artifacts.
- **Files changed** (high level) and any DB/migration/config changes.
- **Acceptance criteria**: concrete items that must be true for merge.
- **Checklist**: tests, linters, docs updated, changelog.

### PR Template (copy this into your PR description)

````
## Summary

One-line summary of the change.

## Purpose / Motivation

Why this change is necessary.

## Related issue(s)

- Fixes #<issue-number> (if applicable)
- Other references / design docs / links

## Proposed changes

- Bullet list of what changed (API, DB, behavior)

## Files modified / created

- `pkg/service/foo.go`: brief note
- `internal/webhook/ensure_group.go`: brief note

## Configuration

- Any new env vars or config needed? If none, state "No configuration changes needed".

## How to test (manual / automated)

1. Steps to run locally or commands, e.g.:
   - `make run` or `go test ./...` or `npm run test`
2. Example requests/commands to run.
3. What to observe.

## Evidence / Logs / API responses

- Paste logs, API responses, or screenshots demonstrating the change:
```
<paste logs and successful responses here>
```

## Acceptance criteria

- [ ] Behavior X works (describe)
- [ ] Tests pass (unit + integration)
- [ ] No regressions for Y

## Checklist before requesting review

- [ ] Code builds and tests pass locally
- [ ] Linting / formatting applied
- [ ] Documentation updated (README, docs)
- [ ] PR size is reviewable (if large, consider splitting)
````


## 4. What makes a *good* PR reads *better*

- **Concise summary**: A single-line Summary that immediately explains the intent.
- **Clear Purpose/Motivation**: Explains *why* the change is required, and ties the code change to standards or business needs (in the example: FHIR standard compliance).
- **Behavior description**: Describes precise behavior changes (how `subject` gets filled for patient vs non-patient).
- **Edge-case handling**: Notes the additional operation (ensuring groups exist) and why it prevents runtime errors.
- **Configuration clarity**: Explicitly states that no configuration changes are required, removing uncertainty for reviewers.
- **Concrete testing steps**: Shows how the author tested the change (run app, inspect logs, query API).
- **Raw evidence included**: Paste of logs and full API responses (JSON bundle) that demonstrate the fix works in both patient and non-patient scenarios.
- **Scope note**: Calls out other components are unaffected and integrate normally (enqueueing, workers), which helps reviewers focus.
- **Readable formatting**: Uses headings, code blocks, and preformatted logs to make scanning quick.

*Example of a good PR:* [https://github.com/konsulin-care/konsulin-api/pull/205](https://github.com/konsulin-care/konsulin-api/pull/205)

## 5. Required QA evidence (what to include in PR)

At minimum include one or more of the following where relevant:
- Unit test(s) covering the main logic.
- Integration test demonstrating the end-to-end behavior.
- Startup or runtime logs proving initialization steps occurred (with timestamps/callers when possible).
- Sample API response(s) showing fields or behavior (paste JSON).
- Screenshots or screencasts for visual changes.
- A short manual test recipe (commands/URLs to hit).


## 6. When resolving an issue: start with a written plan
Before you code, post a short plan comment on the issue describing what you intend to do. A thorough plan includes:

- **Overview**: one paragraph summary of the approach.
- **Problem statement**: what is broken or missing.
- **Proposed flow / implementation**: step-by-step description of the change.
- **Implementation checklist**: concrete tasks you will do (add tests, update docs, etc).
- **Files to modify/create**: high-level list of files.
- **Acceptance criteria**: what reviewers will test to accept the work.

Example skeleton (post as the first comment on the issue):
```
## Overview
Short summary of approach.

## Problem
Describe the problem and why current behavior is undesirable.

## Proposed Implementation
- Step 1: ...
- Step 2: ...
- Step 3: ...

## Files to modify
- `pkg/...`: add X
- `cmd/...`: change Y

## Implementation checklist
- [ ] Unit tests
- [ ] Integration tests
- [ ] Docs updated

## Acceptance criteria
- [ ] Behavior verified (describe how)
```

A good planning comment = faster reviews and fewer surprises.

*Example of a good planning:* [https://github.com/konsulin-care/konsulin-app/issues/277#issuecomment-3342537290](https://github.com/konsulin-care/konsulin-app/issues/277#issuecomment-3342537290)

## 7. Issue creation and required LLM prompt

When submitting an issue, we require contributors to use the LLM prompt below to produce a well-structured issue. Paste your pointers (in any language) into the LLM prompt and then paste the generated markdown into the new GitHub issue.

**Required prompt to use (paste as-is into your LLM tool):**
`````
# Persona

You are an expert software project manager.

# Task

Draft a well-written and information-complete GitHub issue based on the given pointers.

# Rules

1. Your draft must follow agile best practice.
1. You will receive pointers in any language. Your output draft must be in plain business English, no emojis.
1. Always include a coherent user story in the drafted issue.
1. Your draft must be a well-formatted markdown. Return the output in a code block marked with four back ticks.

# Output Format

````markdown

**Title:** The issue title of maximum 20 words

# User Story

{user_story}

# Description

{brief_description}

# Proposed Flow

- [ ] {expected_flow_upon_implementation}

# Reference

- {reference_of_current_implementation_or_other_important_pointers}

**Notes:**

- {further_implementation_notes}
````


You will receive further information from the user.

`````

**How to use**

1. Copy the prompt above into your LLM (or the helper tool we provide).
2. Provide the "pointers" (bullets, logs, requirements) in any language.
3. The LLM will return a ready-to-post Markdown issue in English. Paste that into the GitHub issue body and create the issue.

*Example of a good issue:* [https://github.com/konsulin-care/konsulin-app/issues/263](https://github.com/konsulin-care/konsulin-app/issues/263)


## 8. Labels, reviews, and merge policy

* Add relevant labels to your PR and issue (e.g. `bug`, `feature`, `docs`, `breaking-change`).
* Request reviewers by @-mentioning maintainers or the team.
* PRs must pass CI and receive at least one approving review from a maintainer (or two community reviewers) before merge.
* Maintainers will perform the final merge (usually squash-merge). If you'd like your commits squashed locally, please do so; otherwise maintainers will squash on merge.
* For urgent hotfixes, follow the `hotfix/*` branch workflow and mention `@maintainers` in the PR.


## 9. Tests and CI expectations

* All new behavior should be covered by unit and/or integration tests where feasible.
* Ensure linters and formatters are run and fixed (see project README for commands).
* If your change requires additional CI secrets or infrastructure (e.g., external API keys), document them in the PR and coordinate in Slack.


## 10. Documentation & visual identity

When changes affect UI, visuals, or user-facing copy:

* Follow the Konsulin visual identity and frontend requirement documents.
* Update relevant docs and include screenshots for visual review.

Important links:

* [SuperTokens documentation](https://supertokens.com/docs)
* [Konsulin visual identity](https://docs.google.com/document/d/1apG-LUqRe6lPPN2CxsVA9GHu4C9J5KJzuGfva3BhxhY/edit?tab=t.5wb5sgrkyzhh#heading=h.7heo8npoffl3)
* [Frontend requirement](https://docs.google.com/document/d/1YW4JymNgA1beupeZL7iYyhicoqTxgwqTozGgenI3GI0/edit?tab=t.ig74kdanxape#heading=h.7heo8npoffl3)
* [Backend requirement](https://docs.google.com/document/d/1YW4JymNgA1beupeZL7iYyhicoqTxgwqTozGgenI3GI0/edit?tab=t.0#heading=h.zhxtejv1yk22)
* [FHIR integration](https://docs.google.com/document/d/1apG-LUqRe6lPPN2CxsVA9GHu4C9J5KJzuGfva3BhxhY/edit?tab=t.byrvfyhphjvp#heading=h.7heo8npoffl3)
* [Processing FHIR resource content](https://docs.google.com/document/d/1apG-LUqRe6lPPN2CxsVA9GHu4C9J5KJzuGfva3BhxhY/edit?tab=t.jpa9nm42mi0e#heading=h.4duy8x62xqcw)


## 11. Communication & etiquette

* **Asynchronous:** Use GitHub Issues, PRs, and Discussions for public work and asynchronous conversations.
* **Private:** Use Slack for private coordination and security disclosures.
* Keep PRs focused and small. If a change is large, break it into multiple PRs and link them.
* Be respectful and constructive in reviews. See `CODE_OF_CONDUCT.md` for community expectations.
