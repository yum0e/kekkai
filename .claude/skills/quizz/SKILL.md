---
name: quizz
description: Structured spec interviews using AskUserQuestionTool to validate implementations and surface issues. Use proactively when discussing features or specs.
---

Interview users about specs/features to find gaps, edge cases, and improvements. Use AskUserQuestionTool exclusively - no plain-text questions.

**Invoke proactively** when:
- User discusses a new feature or spec (even before a doc exists)
- User wants to validate an implementation
- User asks about edge cases or potential issues

## Usage

```
/quizz <spec-file>      # existing spec
/quizz <feature-topic>  # new feature discussion
```

## Flow

1. Read spec if exists, otherwise gather context from conversation
2. Interview in phases (1-4 questions per round)
3. Summarize after each round
4. Create or update spec with findings when complete

## Phases

| Phase | Focus |
|-------|-------|
| Status | Implemented vs pending, deviations, blockers |
| Data | Schema, constraints, indexes, types |
| Logic | Edge cases, errors, concurrency |
| Integration | Breaking changes, compatibility |
| Testing | Coverage, untested paths |
| Ops | Monitoring, perf, rollback |
| Future | Tech debt, deferred work |

## AskUserQuestionTool

- Headers: max 12 chars (`Status`, `Edge cases`, `Rollback`)
- Options: 2-4 choices, likely answer first
- `multiSelect: true` when multiple issues can apply
- Severity hints in labels: `(Critical)`, `(Minor)`

## Spec Update

Append or create:

```markdown
---

## Interview Findings (YYYY-MM-DD)

### Critical
- [issue]

### Minor
- [issue]

### Decisions
- [decision]

### Actions
- [ ] [task]
```

## Done When

- All phases covered
- No new concerns
- User confirms via final "Complete?" question
