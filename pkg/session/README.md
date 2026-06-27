# session

Append-only event journal with replay + lifecycle FSM.

## API — Journal

```go
journal, err := session.NewJournal("/path/to/journal.jsonl")
err := journal.Append(event)
events, err := journal.Replay(since time.Time)
```

Events are journaled before dispatch. On restart, unapplied events are replayed.

## API — Lifecycle

```go
fsm := session.NewLifecycleFSM()
fsm.Transition(session.EventSessionStart)
state := fsm.Current()
```

Lifecycle transitions are events, not Reconciler fields. FSM tracks: STARTING → ACTIVE → PAUSED → CLOSED.

## Files

| File | Description |
|------|-------------|
| `journal.go` | Append-only JSONL journal |
| `lifecycle.go` | Lifecycle FSM + event types |
| `types.go` | Session state types |

## Tests

Tests for journal append/replay, lifecycle transitions.
