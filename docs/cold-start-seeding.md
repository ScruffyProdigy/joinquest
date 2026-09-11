# Cold-start seeding: within-game cross-mode skill priors

A player with a settled rating in one mode of a game used to start from the
flat prior in every other mode of that same game, as though the lobby had
never seen them. Cold-start seeding uses the rating they already have — same
rules, same interface, same core loop, differing only in pacing and player
count — to place them better than nothing on their first match in the new
mode.

- Domain logic: `backend/internal/coldstart`
- Storage: `backend/internal/store/cold_start.go`, migration `000068`
- CLI: `backend/cmd/coldstartfit`

**It is off by default.** Nothing is seeded until `COLD_START_SEEDING=true`,
and the point of the flag is that the decision to set it should be read off
the numbers rather than argued (see [Deciding whether to turn it on](#deciding-whether-to-turn-it-on)).

## How a seed is produced

Two numbers, from two different sources:

| | Where it comes from |
|---|---|
| **centre (mu)** | The target mode's population mean, shifted toward the player's standing in the source mode by the measured correlation between the two modes, rescaled between their spreads. This is an ordinary least-squares prediction — a correlation near zero leaves the player at the population mean, which is the flat prior in all but name. |
| **spread (sigma)** | The spread of the errors this same seeding made when it was cross-validated against players who had in fact played both modes — **not** the model's own confidence, and not a chosen constant. Floored, so a seed is never as confident as a played rating. |

Both are measured per **ordered** pair of modes. `arena -> duel` and
`duel -> arena` are two different questions with two different answers: a
tightly-clustered mode predicts a widely-spread one far less precisely than
the reverse.

### Two deliberate asymmetries

**Sigma is biased large.** A seed that is too wide washes out after a few
matches and harms nobody. A seed that is too tight is sticky: a player seeded
wrongly high has to lose repeatedly to come back down, and every one of those
matches is a bad experience for them and for whoever they were matched
against.

**Upward seeds are more conservative than downward ones.** Seeded too high, a
player is outmatched with no way out but losing. Seeded too low, they get an
easy match and climb out within a few. So the part of a seed that sits above
the population mean is shrunk by `UpwardCaution` and the part below it is not.

## Thresholds

All four live as constants in `backend/internal/coldstart`, each with its
reasoning attached.

| Constant | Value | Why |
|---|---|---|
| `MinPairedPlayers` | 30 | A correlation's standard error is roughly `1/sqrt(n-3)` — about 0.19 at n=30, about 0.35 at n=11. At the smaller number a true correlation of zero routinely measures as 0.35, and a seed built on that is confident noise, which is worse than no seed because it looks like a measurement. A pair below this falls back to the flat prior. |
| `ConvergedSigma` | half the flat prior's sigma | How far a rating's uncertainty must have fallen before it counts as evidence. A rating still near the prior is mostly *made of* the prior, so pairing two of them correlates players through what they share rather than through anything either has shown. |
| `SeedSigmaFloor` | half the flat prior's sigma | The tightest a seed may ever be, whatever the backtest says. The measured residual is an average over a population; the guarantee has to hold for the individual in front of us. At this floor a seeded player still moves substantially on their first match, which is what makes a wrong seed self-correcting rather than sticky. |
| `UpwardCaution` | 0.8 | The upward shrink above. Mild on purpose: the regression already pulls every seed toward the mean, and a value low enough to be a real safety margin would flatten every genuinely strong player into the middle. |

A pair is refused outright — flat prior, no seed — when it has too few paired
players, a non-positive correlation, no spread to map from, or a held-out
residual that is not better than the flat prior's. That last clause is what
lets a bad pair be *discovered* rather than debated.

## Why seeding happens on read

A seed is never written into `player_ratings`. That table is a cache of a
replay over `rating_match_inputs`, so a seed written there would be erased by
the next replay — and until it was, it would be indistinguishable from a
rating the player had earned. Seeds are derived on read instead, which needs no
invalidation and stops applying by itself the moment the player's first result
gives them a real rating.

**A stored rating always wins.** Seeding fills a gap; it never overwrites or
softens something a player actually showed.

Every seed served is recorded in `rating_seed_events` — who was seeded, into
what, from where, and on how strong a measurement at that moment. A seed is a
claim the lobby made about someone before watching them play, and when a
mode's matchmaking turns out lopsided that table is where the investigation
starts.

## Deciding whether to turn it on

```bash
cd backend
DATABASE_URL=$(../scripts/db.sh url) go run ./cmd/coldstartfit
```

The command fits every game's mode pairs and prints them. **It does not write
unless you pass `-write`**, so it is safe to point at production to read the
numbers without changing what production serves.

| Flag | Meaning |
|---|---|
| `-game` | Fit one game. With no flag, every game with cached ratings. |
| `-write` | Persist the fit, as the scheduled recompute would. Off by default. |
| `-json` | Machine-readable report. |

Read `residual` against `flat` — the held-out error of the seeding against
that of the flat prior over the same players. The `usable` column applies
every gate above and is the column to act on: a pair marked `no` has been
measured and is not good enough, whatever its correlation looks like.

Turn seeding on when the pairs that matter read `yes` with residuals
meaningfully below flat. Until then the flag stays off and costs nothing.

## Scheduled recompute

Pairs are refitted on a slow tick (`COLD_START_REFIT_INTERVAL`, default 6h) by
a recomputer started in `server.go`. A refit is a full recompute per game:
the fit is over a population's moments and a cross-validated residual, both of
which every new paired player changes, so there is no correct way to fold one
player in incrementally.

**The refit runs whether or not seeding is enabled**, and deliberately. It
costs two queries per game against its own table every few hours, touches no
rating anyone reads, and is the only way the flag ever gets a basis to be
flipped — a deployment that measured nothing until seeding was switched on
would have to switch it on blind.

## Configuration

| Variable | Default | Meaning |
|---|---|---|
| `COLD_START_SEEDING` | unset (off) | `true` serves seeds. Anything else, including unset, serves the flat prior exactly as before. |
| `COLD_START_REFIT_INTERVAL` | `6h` | How often pairs are refitted. An unparseable or non-positive value falls back to the default rather than disabling the refit. |

## Scope

Within one game only. Cross-game and cross-genre transfer need latent factors
and are a separate ticket (JQ-155).

One limit worth knowing: rating is keyed per `(player, game, mode)`, so a
player's rating already averages over whatever roles they played inside that
mode. For a game with asymmetric roles — a saboteur, a traitor — that average
spans roles a player may be unevenly good at, and no cross-mode seed can be
more precise than that average is, however high the correlation climbs. It is
a second reason the sigma floor is not merely defensive.
