# Message Aggregation Design

## Goal

Reduce over-replying to fragmented user input by converting a burst of consecutive raw messages into one utterance before normal processing.

This is not message dropping.

The system still receives every raw message, but the AI-facing processing unit becomes an aggregated utterance rather than a single raw event.

## Current Problem

The bot currently processes every message immediately:

- one raw message
- one inbound pipeline run
- one reply decision

For users who type in fragments, this causes:

- premature replies
- repeated model calls
- weaker intent understanding

Example:

- `我刚到家`
- `今天真的好累`
- `而且外面一直下雨`

These three messages usually form one human utterance, not three independent turns.

## Design

Add a session-level aggregator between:

- raw message receiver
- message processor

New flow:

- raw websocket message
- raw queue
- session aggregator
- aggregated message queue
- inbound pipeline
- message processor

## Aggregation Rules

Messages are grouped by `sessionID`.

For one session:

- the first message starts an active aggregation bucket
- if more messages arrive before the silence window expires, they are merged into the same bucket
- if no new message arrives within the silence window, the bucket is flushed
- if the bucket reaches the max window or max message count, it is flushed immediately

## Three Configurable Properties

### 1. `MESSAGE_AGGREGATE_IDLE_WINDOW_MS`

Meaning:

- how long the system waits for the next message before considering the current utterance complete

Suggested default:

- `2000`

### 2. `MESSAGE_AGGREGATE_MAX_WINDOW_MS`

Meaning:

- maximum total time a bucket can stay open even if the user keeps sending new messages

Suggested default:

- `10000`

### 3. `MESSAGE_AGGREGATE_MAX_MESSAGES`

Meaning:

- maximum number of raw messages allowed in a single utterance bucket

Suggested default:

- `5`

## Output Shape

The aggregated message should preserve:

- latest `MessageID`
- all raw message ids
- all raw message segments
- whether aggregation actually happened
- first message time
- last message time

The AI-facing text is a normalized join of all raw segments, separated by line breaks.

## Why This Works Better

Benefits:

- fewer unnecessary replies
- fewer model calls
- better understanding of split intent
- more human-like pacing

Tradeoff:

- replies become slightly slower because the system waits for a silence window

## Scope

This first version:

- aggregates only target private chat messages
- leaves non-target or control messages on the immediate path
- does not change memory persistence schema

## Future Extensions

- special early-flush heuristics for question marks, images, commands
- per-session adaptive timing
- aggregation-aware metrics and admin visibility
