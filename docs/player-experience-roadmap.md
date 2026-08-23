# Player experience roadmap

**Status:** Planned — catalog polish and queue UX (overlaps optional dev registration metadata)  
**Related:** [developer self-service](./developer-self-service.md) Phase A ✅ shipped · [game-minted launch URLs](./game-minted-launch-urls.md) ✅ shipped

Improvements so players can **pick games they care about** and **get into matches faster** — solo catalog queues, table backfill, and composition modes. Much of the data model for bottlenecks already exists via **`formingGaps`** (Phase B); this spec covers estimates, UX, and catalog polish.

---

## 1. Queue time estimates

### Goal

When someone is waiting (or considering joining), show **how long until a match might fire** — or be honest when we don't know.

### Surfaces

| Surface | Copy direction |
|---------|----------------|
| Game list / mode row | “Usually ~2 min” · “Often fires in under 5 min” |
| Intent banner (waiting) | Same estimate beside “Looking for a group…” |
| Table backfill | “Likely ~4 min to fill remaining roles” |
| No data | “This queue hasn't fired recently — wait time unknown” or “Be the first matchmaker today” |

### Solo catalog queues

**Signals (v1 heuristic, refine later):**

- Rolling median time from “queue reaches fire-ready shape” → `fired` (per `mode_queue_id`, optionally per `queue_path`)
- Recent fire rate (fires per hour, last 24h)
- Current `queuedCount` + `formingGaps` vs historical time-to-fire at similar depth

**Storage (sketch):**

- Aggregate on fire: `forming_match_fired_at - first_ready_at` per queue
- Or periodic job: snapshot queue depth + gaps → correlate with next fire
- Expose GraphQL: `ModeQueue.waitEstimate { label, confidence, secondsP50 }` or human string only

**Confidence levels:**

- `high` — ≥ N fires in last 7d  
- `low` — sparse data  
- `none` — no fires in 30d → “hasn't fired in a while”

### Table backfill

Same estimator, scoped to the table's backfill queue:

- Input: `formingGaps` on `Table` / `MyTableSeat` + catalog waiters for those paths
- Output: “~3 min if one more Support joins” vs unknown

### Player utility

Helps groups decide **whether to wait**, **split roles**, or **open a private table** instead of blind queueing.

---

## 2. Bottleneck callouts & “Join immediately”

### Goal

Highlight **which role is blocking fire** and let players (or a full party) **fill that gap now** — skip waiting for the slow bucket.

**Builds on:** `formingGaps` (`queuePath`, `displayName`, `assigned`, `needed`) already on `ActiveIntent`, `QueueUpdate`, `Table`, `MyTableSeat`.

### Two levels of messaging

Not every missing role gets ⚡. Distinguish **informational gaps** from a **real bottleneck**.

| Level | When | UI |
|-------|------|-----|
| **Gaps (always)** | Any `formingGaps` with `needed > 0` | Plain text: “Need 1 Support, 2 Guesser” — what’s missing to fire |
| **Bottleneck + ⚡** | Real imbalance (see below) | Same gap info + **Join immediately ⚡** on the starving path + priority placement |

**⚡ gate — real bottleneck only**

Show ⚡ (and enable priority placement) only when **both** are true:

1. **Depth on at least one other path** — roughly **2–3 matches’ worth** of players already placed or waiting on non-blocking paths (configurable constant, e.g. `depth ≥ 2 × playersToStart` for the saturated path, or ≥ 2 full rosters on the forming map excluding the starving path).
2. **Starvation on the blocking path** — the critical-path `queuePath` still has `needed > 0` with comparatively few assigned/waiting (the constraint that actually prevents `ReadyToFire`).

Intuition: DPS/Tank queues are **backlogged**; Support is **empty**. Extra Supports unblock multiple would-be matches — that’s worth ⚡. If you only need one Guesser and nobody’s stacked up elsewhere yet, show the gap but **no** ⚡ — there’s no pile-up to optimize.

**Counter-example (no ⚡):** First players of the day — “Need 1 Tank, 4 DPS” with nobody on the map. Informational gaps only.

**Example (⚡):** Enough DPS/Tank for ~3 fires on the forming map; Support still `needed ≥ 1`. “**12 players waiting on DPS** — match blocked on **Support**. Join as Support ⚡”

Tune threshold (2 vs 3 “games worth”) from telemetry; expose as server constant.

### UX when ⚡ is active

- Primary CTA: **Join immediately ⚡** on the **starving** bottleneck path only (catalog + tables)
- Composition: joins as that `queuePath` with priority placement
- Other paths with `needed > 0` but no imbalance → gap text only, normal join buttons

**Catalog:** Game list + intent banner — ⚡ only when bottleneck gate passes.

**Tables (same ⚡, room context):** Tables are where friend groups coordinate; bottlenecks during **backfill** should be just as visible and actionable as solo catalog queues.

| Surface | Who | ⚡ behavior |
|---------|-----|-------------|
| **TableCard** (backfill active) | King + seated players + room visitors | ⚡ only when catalog forming match passes bottleneck gate; label names starving role |
| **Intent banner** (table seat / backfill) | Same room members | Mirror TableCard bottleneck + ⚡ |
| **Room chat / share** | King | Optional “We need Support” copy tied to ⚡ link for invitees |

**Table-specific goals:**

1. **Help the group restructure** — “We’re blocked on Support” → someone **sits** Support at the table, or a friend **joins catalog as Support** with priority placement (table-seeded forming match).
2. **Clear bottlenecks faster** — ⚡ on the blocking path for anyone in the room who can fill it (sit empty seat if role maps to `seatKey`, or `startTableBackfill` / queue as that path).
3. **Optimize before waiting** — Show all `formingGaps` on the card; highlight **critical path** with ⚡; dim non-bottleneck paths (“DPS — covered by your table”).

**TableCard UX (sketch — bottleneck active):**

```text
Word Hunt · backfill active
Need 1 Support, 2 Guesser to start
~18 players waiting on Guesser roles · blocked on Support
[ Join as Support ⚡ ]

Your table: 1 Clue Giver seated · 1 Guesser seated
```

**TableCard UX (sketch — gaps only, no ⚡):**

```text
Need 1 Clue Giver, 4 Guesser to start
[ Look for group ]
```

- If the viewer can **sit** an open seat for the bottleneck role → ⚡ = sit + backfill participates as that path (or re-seat flow).
- If the viewer is **not seated** → ⚡ = queue into table’s forming match as that path (priority placement).
- King still has **Look for group** for general backfill; ⚡ is the **surgical** CTA for the blocking role.

**Parity:** Same lightning emoji and “Join immediately” language on **catalog** and **tables** so players learn one pattern.

### Priority placement on the bottleneck path

**Mental model:** Fire is blocked by the **critical path** — whichever `queuePath` still has `needed > 0` and prevents `ReadyToFire`. Everyone waiting on other roles is already stuck behind that constraint. Speeding up the bottleneck **shortens wait for all roles**, not just the joiner.

**Bottleneck is not “who has the longest FIFO.”** If four solo Supports are waiting but Support still shows `needed > 0`, those four aren’t on the map yet (or the gap is elsewhere) — Support isn’t what’s blocking. If Support is truly saturated on the map (`needed = 0`), the bottleneck is DPS, Tank, Guesser, etc. A new Support isn’t “jumping” four Supports; they’re waiting on the real blocker like everyone else.

**Policy: prefer joiners on the bottleneck path (when ⚡ gate passes).**

When the server marks `bottleneckPath` (starving path + depth threshold met):

- **Join immediately ⚡** → join as that `queuePath` with **priority placement** on the forming map for that path
- Applies to solos and parties, full or partial (party brings 1 Support when 2 needed still helps — place them first on Support)
- Other paths (DPS, Tank, …) are unchanged; they remain blocked until Support (or whichever path) clears anyway — prioritizing Support joiners does **not** slow saturated DPS/Tank queues

**Example:** Three matches’ worth of DPS and Tank are waiting on the map, but fire is blocked on Support. Bringing any Support (solo or party) to the **front of Support placement** only affects Support ordering. DPS/Tank waiters are still blocked on Support; earlier Support placement **helps** them fire sooner.

**Table backfill:** Seated friends pin non-bottleneck roles; ⚡ on TableCard + intent banner recruits or prioritizes joiners for the blocking path (in-room sit, re-seat, or catalog join with priority). Table pins + catalog priority placement reuse Phase B forming logic.

**Copy:** “**Support** is what’s holding everyone up — join as Support ⚡” not “skip the line.”

### Relation to estimates

Bottleneck + estimate together: “**Support** is the hold-up · usually ~2 min once someone queues”

---

## 3. Game catalog presentation

### Problem

Players only see a **title** today. They can't tell genre, vibe, player count, or whether the game is active.

### Goal

Rich enough cards to **choose what to play**; optional depth for detail view.

### Fields (tiered)

| Tier | Fields | Source |
|------|--------|--------|
| **Card (required for good UX)** | Icon, short description (1–2 lines), tags (genre/mood/player count band) | DB / admin / future dev registration |
| **Detail page (optional)** | Long description, screenshots (carousel), “last match fired”, queue estimate | DB + live stats |
| **From manifest** | Mode names, seat layout, min/max players | Already synced — surface on detail |

### Schema (sketch)

```text
games
  icon_url          (or icon_key → CDN catalog)
  short_description (card)
  long_description  (detail, markdown?)
  tags              (text[] or join table: coop, competitive, 2v2, party, …)

game_media
  game_id, kind (screenshot | trailer), url, sort_order
```

Developer self-service registration can **collect short description + tags** early; icons/screenshots align with [developer-self-service.md](./developer-self-service.md) optional pre-work.

### UI (sketch)

- **GameCard:** hero art background, per-game accent color, badge pills (genre/mode + player count), title, short description; no wait-estimate data yet (tracked separately, JQ-59)
- **Game detail** (modal or route `/games/:slug`): screenshots, long description, modes, “who's waiting” / gap summary per queue path
- Mobile: card height bounded; tags max 3 visible

### Empty states

- No icon → generated placeholder or monogram from title  
- No description → “No description yet” (dev-facing nudge in dashboard later)

---

## Phasing (suggested)

| Order | Item | Notes |
|-------|------|-------|
| 1 | ~~[Game-minted launch URLs](./game-minted-launch-urls.md)~~ | ✅ Shipped |
| 2 | Catalog card UI (icon, short description, tags) | DB + admin seed; no estimator yet |
| 3 | Bottleneck UI + Join immediately ⚡ | Uses existing `formingGaps` |
| 4 | Wait estimates (solo + table) | Needs fire telemetry |
| 5 | Priority placement on bottleneck path (solo + party) | Store: prefer placement on blocking `queuePath` |
| 6 | Detail page + screenshots | Nice-to-have before dev self-service |
| 7 | Developer self-service Phase A | Devs supply metadata at register |

Items 2–4 materially improve **player** experience on current catalog; item 7 improves **supply** of games with metadata.

---

## Open questions

- [ ] Estimate copy: show range (“2–5 min”) vs single number vs qualitative only?  
- [ ] Priority placement: same rule for catalog solo, party tree, and table backfill?  
- [ ] Tags: fixed taxonomy vs freeform?  
- [ ] Screenshots: host on JoinQuest CDN vs dev URLs only?  
- [ ] Exact depth threshold: 2 vs 3 “games worth”; count placed only vs placed + waiting?  
- [ ] GraphQL: `bottleneckPath` + `bottleneckDepth` on `ActiveIntent` / `Table` / `QueueUpdate`?  

---

## Success criteria

- [ ] Waiting player sees estimate or honest “unknown” on intent banner  
- [ ] ⚡ hidden when only informational gaps (no 2–3× depth imbalance)  
- [ ] Composition game shows ⚡ join-as-Support only when DPS/Tank depth threshold met  
- [ ] Table backfill shows which role blocks start + estimate when data exists  
- [ ] TableCard shows ⚡ per bottleneck path; in-room player can sit or queue as that role  
- [ ] Catalog card shows icon, blurb, and at least one tag for seeded games  
- [ ] Support joiner with ⚡ is priority-placed on bottleneck path; DPS/Tank waiters fire sooner, not later
