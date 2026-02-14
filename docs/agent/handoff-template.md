# Handoff Template

Use this structure for thread-close updates and PR summaries.

```text
Scope: <week/milestone and task IDs targeted>
Changes: <files/services changed>
Acceptance criteria status: <pass/fail/partial by criterion>
Risks/issues: <blockers, tradeoffs, pending decisions>
Next step: <single smallest actionable next task>
```

## Example

```text
Scope: Week 1, W1-004 to W1-006
Changes: Added Kafka contract schema; implemented POST /v1/jobs/weather in API
Acceptance criteria status: Partial - submission path working; end-to-end flow not yet complete
Risks/issues: Weather provider key handling still pending final env strategy
Next step: Implement W1-007 initial Redis queued state write on submit
```
