# Player activity events

An append-only record of what players did, captured so that a later analysis can
work out which signals predict how hard a game is to get into.

This document is the reference for what is recorded, what the platform can honestly
claim to observe, and how long any of it is kept. Introduced in JQ-143.

## Why it exists

We want to tell players how difficult a game is along two axes: the **floor** (how
hard it is to start) and the **ceiling** (how much room there is to master it). The
floor matters more, and nobody yet knows which measurable signals track it.

So the platform records broadly now and correlates later. Deciding which signals mean
"easy to learn" is deliberately a separate piece of work — this is only the recording.
A signal nobody wrote down cannot be correlated after the fact.

## The envelope

Everything lands in one table, `player_activity_events`:

| Column | Meaning |
|---|---|
| `event_type` | What happened. Free text, not an enum |
| `source` | Who observed it. `lobby` today |
| `user_id` | The lobby user id — the only identifying value permitted anywhere on the row |
| `game_id`, `mode_key`, `session_id` | What it was about. All optional |
| `occurred_at` | When the thing happened |
| `recorded_at` | When the row was written. The gap is instrumentation lag |
| `payload` | Whatever that event type knows |

**Adding a new signal requires no migration.** Add a constant in
`backend/internal/activity/event.go` and emit it. That is the property the whole
design is arranged around: no schema decision had to be right in advance, which is
what makes "record everything" safe rather than a liability.

The table has **no foreign keys**. That is deliberate — see *Safety*, below.

### Event types today

| Type | Emitted from |
|---|---|
| `queue_joined` | `JoinModeQueueWithOptions`, after commit |
| `queue_abandoned` | `LeaveModeQueue` (`reason: left`) and `EvictDisconnectedWaitingEntry` (`reason: evicted`) |
| `match_started` | Session creation — `StartTable` and the matchmaking reconcile |
| `match_provisioned` | `SessionProvisionComplete` |
| `launch_url_requested` | `GetSessionParticipantLaunchURLBase` |
| `match_finished` | `RecordPlayerFinish`, carrying the finish reason |
| `match_completed` | `CompleteSession` |

Two of those sit where they do for reasons worth keeping:

- **`match_started` comes from session creation, not from a room table being stamped
  started.** A matchmade session can involve no tables at all (every player joined
  from the catalog) or several (a 3v3 formed out of two rooms), so hooking the table
  stamp would miss players in the first case and double-count them in the second.
- **`queue_abandoned` comes from the server's eviction decision, not the socket
  edge.** A client that drops and reconnects inside the grace window never abandoned
  anything. Counting socket edges would read a flaky network as a game being hard to
  get into — the exact signal this data exists to measure honestly — and the client's
  reconnect budget is ~87.5s (JQ-283), so that noise is not small.

## What the platform can actually observe

**Read this before trusting any number derived from these events.**

The lobby sees a player join a queue, sees a match form, sees provisioning finish, and
sees a player ask for their launch URL. After that it is blind until the game reports
something back.

| Step | Observable? |
|---|---|
| Queue join | Yes |
| Queue abandon | Yes, and deliberate leaves are distinguishable from evictions |
| Match formed | Yes |
| Provisioning complete | Yes |
| Launch URL issued and fetched | Yes — but this is **not** proof of entry |
| **Player entered the game** | **No. Not observable at all today** |
| Match completed / finish reason | Only when the game reported one |
| Second match | Yes |

A launch URL being fetched is an **upper bound** on players reaching the game. They
may never have opened it, or opened it and bounced off a loading screen. There is no
column for confirmed entry, rather than a column that would quietly be a guess.

A match the game never reported on has an **unknown** outcome — not a failed one and
not a completed one. `game_playtest_summary.matches_outcome_unknown` counts
exactly those, and it is a real answer rather than a gap.

The temptation with a funnel is to infer the missing half so every column has a number
in it. That would make the platform's blind spots invisible at precisely the moment
someone is deciding whether a game is hard to get into. **Unknown stays unknown.**

## Clocks

**`occurred_at` is not always on the same clock, and the clocks disagree.**

Postgres's `NOW()` and the Go process clock have been observed several seconds apart
**in both directions** on the local Docker stack. This is not a theoretical caveat —
it is the root cause of a real bug elsewhere in this repo, where a hold window
compared a database-stamped timestamp against `time.Since` and stopped expiring.

| Column | Clock |
|---|---|
| `occurred_at` on `match_started` | Database (`game_sessions.started_at`, stamped at transaction start) |
| `occurred_at` on every other event | The emitting process |
| `recorded_at` | Always the database — the writer never supplies it |

**The rule for analysis:** subtracting `occurred_at` across two event types that use
different clocks gives you a duration *plus an unknown skew*, not a duration.

Comparing like with like is sound. `player_activity_first_match.gap_to_second_match`
subtracts two `match_started` values — both database-stamped — and is correct for
exactly that reason. Anything new that crosses the boundary needs to account for it,
or say plainly that it does not.

`recorded_at − occurred_at` is *approximately* the instrumentation's own lag, and only
approximately, for the same reason. It is pinned to the database clock precisely so a
row can never appear to have been written before the event it records, which would be
nonsense on an append-only table.

Database time is the right default in a multi-pod deployment — every API process has
its own clock, and only the database's is shared — so the fix for a future
cross-clock comparison is to move the other side onto the database, not to move
`match_started` off it.

## Querying it

Two views, so analysis does not mean ad-hoc SQL against production tables:

- **`player_activity_first_match`** — per player and game: their first match, whether
  the outcome was observed, the finish reason, and whether and when they came back.
  This is where the floor hypothesis lives. A `FORFEIT` or `DISCONNECT` on a player's
  first-ever match of a game is close to a direct reading of that game's floor.
- **`game_playtest_summary`** — per game: the funnel a developer wants after a
  playtest, with the blind spots above left visible.

`game_playtest_summary` is the intended source for the developer-facing analytics in
JQ-9 and JQ-166. The dashboard UI and any game-emitted telemetry contract are separate
pieces of work; this provides the numbers, not the screen.

### Reading the summary honestly

Every column names its own unit, because the two units are easy to confuse and produce
a plausible wrong answer when they are:

- **Player-scale** — `queue_joins`, `queue_abandons`, `player_match_starts`,
  `player_finishes_reported`, `finishes_forfeit`, `finishes_disconnect`. One per player
  action.
- **Match-scale** — `matches_started`, `matches_provisioned`, `matches_completed`,
  `matches_outcome_unknown`. One per session, however many players were in it.
- **Distinct players** — `distinct_players`, `players_requesting_launch`,
  `players_with_second_match`.

Dividing `matches_completed` by `player_match_starts` looks like a completion rate and
is really the reciprocal of the party size. Compare like with like.

Also:

- `players_requesting_launch` is an upper bound on entry, never a confirmation.
- `distinct_users` in `player_activity_daily` is **per day** and is not summable
  across days — a player active on Monday and Tuesday counts once in each.
- `players_with_second_match` counts observed second match starts, not rematch offers
  shown or regroup intent.

## Retention

**Raw events are kept for 180 days.** Aggregates are kept indefinitely.

The window is `store.DefaultActivityRetention`, overridable per deployment by
`ACTIVITY_EVENT_RETENTION` on the sweep CronJob
(`k8s/jobs/player-activity-sweep.yaml`). If you change one, change the other and this
document with them.

The tension the window resolves: longitudinal analysis gets wanted at a point when
volume does not yet justify having kept everything, and deleted is deleted. So the
sweep preserves the answers before removing the rows:

1. `player_activity_daily` — the funnel per game, mode, day. Enough to ask "is this
   game getting easier to get into over time" long after the raw events are gone.
2. `player_first_match_summary` — the floor signal at full per-player resolution,
   because aggregating it into a daily count would destroy exactly the correlation a
   later analysis needs. It grows with players rather than with play, so it is
   affordable to keep forever.

**What is given up:** the ability to ask a *new* question of raw events older than 180
days. That is a real loss and it is the accepted one.

The sweep runs rollup → summary → delete, and a failure in either of the first two
aborts the run. Unlike the queue and room sweeps, whose passes are independent, these
are strictly ordered: raw rows must not be dropped on a run that failed to fold them
up first.

```bash
# What would be deleted, without deleting it
./activitysweep -dry-run
```

## Privacy

Payloads carry **no personally identifying data beyond the lobby user id**, which has
its own column.

This is enforced rather than documented-and-hoped-for: `sanitizePayload` strips keys
naming personal data (`email`, `display_name`, `client_ip`, `user_agent`, tokens,
addresses, phone numbers) at `Record`, before anything is queued. Matching is on whole
tokens rather than substrings — `participant` contains `ip`, and a substring rule
would silently eat `participant_count`.

The event still records with the rest of its payload intact. Dropping a whole signal
because one field was careless would trade a privacy problem for a data problem.

## Safety

**Recording can never block or fail a player-facing action.** This is the constraint
the package is built around, and it shows up in three places:

1. `activity.Recorder.Record` takes no context and returns no error. A caller on the
   matchmaking path is structurally incapable of waiting on it or mishandling its
   failure.
2. All real work happens on a background goroutine. When the buffer fills — a slow or
   absent database — events are **dropped and counted**, never queued into
   backpressure. `Writer.Dropped()` is non-zero exactly when the stream has holes,
   which an analysis should know before trusting event counts.
3. The table has no foreign keys. A foreign key would make an instrumentation INSERT
   fail when its referent is gone, and make deleting a user wait on this table.

Every emit call also sits **after** its surrounding transaction commits. A transaction
that rolled back did not happen and must not be recorded, and an event emitted inside
one could abort the match it was describing.

The cost, stated plainly: events are not durable when `Record` returns. A crash loses
what is buffered. That is the right trade for data whose purpose is statistical
correlation — a missing fraction of a percent changes no conclusion, while a
matchmaking path that can hang on an analytics write is a live outage.

## Guest merges

`MergeUserInto` carries both the raw events and the first-match summary onto the
surviving account.

This is not a detail. Nearly every player arrives as a guest, so a first-ever match —
the sharpest floor reading there is — is almost always recorded against an identity
the player has since stopped being. Without the carry, the data would look like a
catalog of players who each tried one game once and never came back.

On a summary collision the **earlier** first match wins, which is the opposite of how
`game_session_participants` resolves one. A player's first match is a fact about when
they first met the game; keeping the account's later row would move it forward in time
and understate how long they had been bouncing off that game before signing up.

## Open questions

- **Where the events live.** Postgres is the right answer until volume says otherwise,
  and this deliberately adds no infrastructure ahead of that.
- **Whether games can emit their own events.** A game knows things the lobby never
  will — that a player fumbled a tutorial, say. That is a protocol addition and a
  later ticket, but the envelope does not preclude it: `source` exists for exactly
  this, and a new event type needs no migration.
