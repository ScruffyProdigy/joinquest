# Backtesting rating models

The rating engine is built so the choice of model is reversible:
`rating_match_inputs` is the source of truth and the stored ratings are a
derived cache, so any engine can be re-run over the same history (see
`backend/internal/rating`). The backtest harness turns that property into a
number, so questions about the engine can be settled by measurement instead of
by taste.

- Harness: `backend/internal/ratingbacktest`
- CLI: `backend/cmd/backtest` — score models against each other
- CLI: `backend/cmd/ratingbeta` — fit a mode's beta from its own history

## Running it

```bash
cd backend
DATABASE_URL=$(../scripts/db.sh url) go run ./cmd/backtest
```

```bash
go run ./cmd/backtest -game <uuid> -mode ranked -train-fraction 0.7
```

```bash
go run ./cmd/backtest -json | jq '.Modes[].Engines'
```

| Flag | Meaning |
|---|---|
| `-game`, `-mode` | Narrow the run. With neither, every `(game, mode)` with retained history is scored. `-mode` needs `-game`. |
| `-engine` | Comma-separated models to score **side by side over identical history**. One model exists today (`plackett-luce`); a second (JQ-231) is what makes the comparison interesting. |
| `-train-fraction` | Share of each mode's history used for training. Default `0.8`. |
| `-train-matches` | Exact training prefix; overrides `-train-fraction`. Use it when history has grown between two runs you want to compare. |
| `-json` | Machine-readable report. |

**It never writes.** The harness holds the store through
`ratingbacktest.Source`, a read-only interface with no `SaveAll` — running it
against production cannot touch `player_ratings` or `nonplayer_ratings`, and
that is enforced by the type rather than by care.

## How the score is produced

Evaluation is walk-forward (prequential): history splits into a training prefix
and a held-out suffix, and each held-out match is forecast from the ratings as
they stood at that moment, scored, and only then fed to the engine. Freezing
the ratings at the end of training instead would penalise later matches for
staleness — measuring the length of the suffix rather than the model.

- **acc** — share of side-pairs the engine ordered correctly, over pairs that
  actually placed differently. `0.5` is chance; **below** `0.5` means the model
  is ordering sides backwards, which is a different failure from knowing
  nothing and worth being able to tell apart. Sides the engine rates dead level
  count as half.
- **logloss**, **brier** — mean error of the forecast win probabilities against
  the actual winner. Lower is better; both punish confident mistakes in a way
  accuracy cannot. They need an engine that implements `rating.Predictor`, and
  print `-` for one that does not rather than a zero that reads like a perfect
  score. Matches with a shared first place have no single outcome to score
  against and are excluded (the count is in `ProbMatches`).
- **scored** — how many held-out matches are behind the number. It travels with
  every score on purpose: an accuracy over four matches and one over forty
  thousand look identical and are not the same evidence.

## Read the modifiers block

Every mode's scores print above a `modifiers:` block — the same identifiability
statistic `ReplayMode` produces, from
`rating.ModifierIdentifiability`. A modifier only means anything if it varies
*within* players. If every player who ever played White only ever played White,
their skill and the seat advantage are perfectly confounded and the split
between them is arbitrary; the numbers stay plausible and stop being true.

```
  modifiers:
    seat:Black                 matches=20     values=2   players-with-multiple-values=7
    seat:White                 matches=20     values=2   players-with-multiple-values=0  ** confounded with player skill **
```

`players-with-multiple-values=0` is the warning. A comparison between two
engines over history like that is suspect no matter how clean the scores look,
which is why the caveat prints beside them and not somewhere else.

## Why there may be nothing to score yet

The harness reads real match history. Until games have been played there is
none, and the tool says so rather than inventing a number. It exists ahead of
the data deliberately: a harness discovered to be wrong after a year of history
has accumulated is worse than one built before the first match.

Its own correctness does not depend on real history — `ratingbacktest`'s tests
generate synthetic history from a known latent-skill model and prove the
harness ranks a model that recovers skill above one that cannot learn, above
one that learns backwards. A harness that always reported a tie would be worse
than no harness, because it would look like evidence.

## Fitting beta per mode

Beta is the engine's performance variance: the distance in skill that buys
about an 80% chance of winning, and so a statement about **how much a skill gap
predicts the result at all**. A small beta describes a mode where the better
player nearly always wins; a large one describes a mode that is mostly luck.

JQ-139 shipped it as a single constant for the whole catalog (`rating.DefaultBeta`,
half the prior sigma). That asserts skill separates players exactly as well in
an RPSLR duel as in a high-luck party game, which is certainly wrong across a
catalog this varied. `cmd/ratingbeta` measures it per mode instead.

```bash
cd backend
DATABASE_URL=$(../scripts/db.sh url) go run ./cmd/ratingbeta          # report only
DATABASE_URL=$(../scripts/db.sh url) go run ./cmd/ratingbeta -apply   # record what qualifies
```

| Flag | Meaning |
|---|---|
| `-game`, `-mode` | Narrow the run, as for `backtest`. |
| `-train-fraction`, `-train-matches` | Where history splits. Same meaning as `backtest`. |
| `-min-matches` | Scored held-out matches required before a fit is recorded. Default `200`. |
| `-multipliers` | Candidate betas as multiples of the default, replacing the standard grid. |
| `-apply` | Write every qualifying estimate. Without it nothing is written. |
| `-json` | Machine-readable report. |

The method is the harness's own prequential walk, run once per candidate beta:
each candidate rates the training prefix, then forecasts and rates its way
through the held-out suffix, and the candidate with the lowest held-out log
loss wins. Beta is chosen on how well it predicts, because predicting is what
beta is for. Running each candidate end to end under its own constants is what
keeps this from being circular — beta does not only tune the forecast, it
changes the ratings the forecast is made from.

The grid is geometric and coarse (0.125x to 8x the default) because beta is a
scale, and because a finer grid would report differences no mode here has the
history to support. The job is telling a skill-driven mode from a luck-driven
one, not resolving beta to three digits.

### What the report will not do

- **Fit a thin mode.** Below `-min-matches` scored held-out matches the mode
  keeps the default and is reported as **unmeasured**, with the sample size
  that fell short. An argmin over a few dozen matches is noise with a decimal
  point. The floor is a number picked to be revisited (`ratingbacktest.MinProbMatches`),
  not a derived quantity — which is why every estimate carries its sample size
  regardless.
- **Hide a bound.** A winner sitting at either end of the grid is flagged: the
  value that would actually minimise log loss may lie outside it.
- **Hide a flat curve.** Every candidate prints, not just the winner. A curve
  that barely moves across a 64-fold change in beta means this history cannot
  tell the two kinds of mode apart at all, and its minimum means nothing.

### Recording one changes the mode's history

A recorded beta lands in `mode_rating_constants` and changes the engine's
identity for that mode — `weng-lin/plackett-luce@1` becomes
`weng-lin/plackett-luce@1+beta=<value>`. Ratings computed under two different
betas are not on one scale and cannot be compared, so the mode's entire history
is replayed under the new value rather than continued: `ListModesNeedingReplay`
treats cached ratings carrying the wrong engine id as exactly as stale as
ratings behind their inputs, and the next rating sweep picks the mode up.

Nothing a player sees moves at the moment of the write. It moves when that
replay runs.

The default beta appends nothing to the engine id, so the modes that have not
been measured — most of them — keep the identity they already have and are not
replayed for a change that did not happen.

### What it is for

Matchmaking cannot currently ask how hard it should try for a given mode. A
mode where skill barely moves the result should spend its budget on queue time
and group fit; strict skill matching there costs waiting and buys nothing a
player would notice. Whether a measured beta should feed the matchmaking
tolerance automatically, or stay an input a human reads, is still open
(JQ-227) — today it is the latter.

## Related

- `backend/internal/rating` — the engine interface, replay, and the
  `Predictor` capability the probabilistic metrics need
- [Seat templates & LFG](seat-templates-and-matchmaking.md) — where seat
  modifiers come from
