# Vendored: intinig/go-openskill

Upstream: https://github.com/intinig/go-openskill
Commit:   7fc4e0aaabbce02fb84c426e8d73c0c8e9d3d3c0 (2026-08-25, "Merge pull
          request #13 ... Bump github.com/montanaflynn/stats ...")
License:  MIT (see LICENSE)

Vendored rather than imported because JQ-139 requires rating updates to be
deterministic and replayable, which cannot be guaranteed for numerics we have
not read.

## Layout

Upstream's own directory/package split (`ptr`, `types`, `unwind`, `util`,
`models`, `rating`, `test`) is preserved rather than flattened into one
package. The point of vendoring is that our copy stays diffable against a
known upstream commit — flattening would mean hand-editing every file, which
is exactly the kind of change that can perturb numerics we vendored precisely
because we did not want surprises in them. A larger directory tree is cheaper
than a larger semantic diff.

## Scope

Only the core `New` / `NewWithOptions` / `Rate` path is vendored:
`ptr`, `types`, `unwind`, `util`, `models`, and `rating/{rate,rating}.go`
(plus their tests) and the `test` fixtures they share.

**Deliberately not vendored:** `rating/ordinal.go` (`Ordinal`, `TeamOrdinal`)
and `rating/predict.go` (`PredictWin`, `PredictDraw`, `PredictRank`), along
with their test files `rating/ordinal_test.go` and `rating/predict_test.go`.
`Ordinal` exists to display a rating to a player, and showing ratings to
players is an explicit non-goal of this ticket. `Predict*` exists to inform
matchmaking, also an explicit non-goal. Neither sits in the deterministic
replay path. Pulling `gonum.org/v1/gonum` and `github.com/montanaflynn/stats`
into the backend's dependency graph to support two functions nobody calls was
judged the wrong trade. If `Ordinal` is ever needed, it is `mu - 3*sigma` and
can be added deliberately then, without dragging `Predict*` or either
third-party numerics library along with it.

This was a scope decision, not an oversight — the next person should not
assume it was missed.

## Local changes

- Import paths rewritten from `github.com/intinig/go-openskill/...` to
  `github.com/scruffyprodigy/joinquest/internal/rating/openskill/...`.
- `github.com/matryer/is` added to `backend/go.mod` as a test-only dependency
  so upstream's own tests could be kept verbatim (they are our inherited
  coverage; rewriting them to stdlib `testing` risked quietly changing what
  they assert). It appears only in `_test.go` files.
- No other source changes. No determinism fix was needed (see audit below).

## Determinism audit

Performed against the vendored subset before committing:

- `grep -rn "math/rand"` over the vendored tree: zero hits in production code.
  (Upstream's `rating/ordinal_test.go`, which used `math/rand`, was not
  vendored — see Scope above.)
- `grep -rn '"time"'` over the vendored tree: zero hits.
- Map iteration in a computation path: one instance, `util.Rankings()` builds
  `origMap := map[int][]int{}` then ranges over it to collect keys — but
  immediately calls `sort.Ints(uniques)` before `uniques` is used for
  anything, so map iteration order never reaches the output. Examined and
  cleared, not missed. No other `range` over a map anywhere in the vendored
  package (all other `range` loops are over slices).

No `math/rand` or `time` in production paths, and no determinism defect
found in the vendored `Rate`/`NewWithOptions` path.
