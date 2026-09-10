package store

import (
	"context"
	"database/sql"
	"sort"
	"strings"

	"github.com/google/uuid"
	"github.com/scruffyprodigy/joinquest/internal/dispersion"
	"github.com/scruffyprodigy/joinquest/internal/rating"
)

// improveDeferredLobbyTx exchanges one seated party for a waiting party of the
// same shape, when doing so tightens the lobby.
//
// Without this a deferral is pure cost. Placement is greedy and first-come:
// syncWaitingPartiesOnFormingTx seats each waiting party the moment it fits,
// and nothing ever moves a player already on the map. So holding the fire
// accumulates candidates in a pool the solver never looks at, the budget
// expires on the identical lobby, and every player paid the wait for nothing.
// Choice has to be spent as well as bought.
//
// It is deliberately narrower than the relocation Phase C describes. A swap
// substitutes one party into seats that are already validly filled, so the
// placement solver never re-runs and no player moves to a different seat. That
// narrowness is what makes it safe to land now, and it is also what enforces
// the party rule for free: a party is atomic on both sides of a swap, so
// nothing here can split one.
//
// Two properties worth stating because they are load-bearing rather than
// incidental:
//
//   - A swap must strictly tighten the lobby. Equal is not enough. That makes
//     the sequence of swaps monotone, so a displaced party can never be swapped
//     straight back in and two candidates cannot trade the same seat forever.
//   - The displaced party keeps its waiting row untouched, joined_at included.
//     It loses this table, never its place in line -- which matters, because a
//     swap that cost queue position would penalise precisely the players the
//     dispersion objective exists to serve.
func (s *Store) improveDeferredLobbyTx(
	ctx context.Context,
	tx *sql.Tx,
	joinCtx *ModeQueueJoinContext,
	fm *FormingMatch,
) (bool, error) {
	assignments, err := s.ListFormingAssignmentsTx(ctx, tx, fm.ID)
	if err != nil {
		return false, err
	}

	seatsByParty := map[uuid.UUID][]FormingAssignment{}
	seated := make([]uuid.UUID, 0, len(assignments))
	onMap := map[uuid.UUID]bool{}
	for _, a := range assignments {
		if a.UserID == nil || a.PartyID == nil {
			continue
		}
		onMap[*a.PartyID] = true
		if a.TableID != nil {
			// A table is a room of people who chose each other, and its seating
			// is not this mechanism's to rearrange.
			continue
		}
		seatsByParty[*a.PartyID] = append(seatsByParty[*a.PartyID], a)
		seated = append(seated, *a.UserID)
	}
	if len(seatsByParty) == 0 {
		return false, nil
	}

	// Read the waiting pool fresh rather than reusing the reconcile's earlier
	// snapshot. That snapshot is taken before syncWaitingPartiesOnFormingTx
	// runs, so every party seated on this very tick still looks unplaced in it
	// -- and a candidate list built from it would happily "swap" a seated
	// player for themselves.
	waiting, err := listWaitingModeQueueEntriesTx(ctx, tx, joinCtx.ModeQueue.ID)
	if err != nil {
		return false, err
	}

	candidates, err := s.reselectionCandidatesTx(ctx, tx, waiting, onMap)
	if err != nil {
		return false, err
	}
	if len(candidates) == 0 {
		return false, nil
	}

	everyone := append([]uuid.UUID{}, seated...)
	for _, c := range candidates {
		for _, m := range c.members {
			everyone = append(everyone, m.UserID)
		}
	}
	mu, err := s.muByUserTx(ctx, joinCtx, everyone)
	if err != nil {
		return false, err
	}

	current := make([]float64, 0, len(seated))
	for _, id := range seated {
		current = append(current, mu[id])
	}
	best := dispersion.Of(current)

	var (
		bestOut  uuid.UUID
		bestIn   *reselectionCandidate
		bestSeat map[string]uuid.UUID
	)
	// Sorted so an improvement is picked deterministically when two candidates
	// tie -- a solve that depends on map iteration order is a solve that cannot
	// be reproduced from a bug report.
	for _, partyID := range sortedPartyIDs(seatsByParty) {
		seats := seatsByParty[partyID]
		for i := range candidates {
			candidate := &candidates[i]
			seatByUser, ok := pairBySeatPath(seats, candidate.members)
			if !ok {
				continue
			}
			trial := make([]float64, 0, len(seated))
			outgoing := map[uuid.UUID]bool{}
			for _, seat := range seats {
				outgoing[*seat.UserID] = true
			}
			for _, id := range seated {
				if !outgoing[id] {
					trial = append(trial, mu[id])
				}
			}
			for _, m := range candidate.members {
				trial = append(trial, mu[m.UserID])
			}
			if spread := dispersion.Of(trial); spread.Tighter(best) {
				best = spread
				bestOut = partyID
				bestIn = candidate
				bestSeat = seatByUser
			}
		}
	}
	if bestIn == nil {
		return false, nil
	}

	return true, s.applyReselectionTx(ctx, tx, fm.ID, bestOut, bestIn, bestSeat)
}

// reselectionCandidate is a waiting party that could take somebody's seats.
type reselectionCandidate struct {
	partyID uuid.UUID
	members []JoinPartyMemberInput
}

// reselectionCandidatesTx collects the waiting parties not already on this map.
//
// The same exclusions the placement path applies: a player who is not watching
// is not seated, here for the same reason as there -- seating somebody who has
// wandered off trades a lobby that is merely mismatched for one that stalls.
func (s *Store) reselectionCandidatesTx(
	ctx context.Context,
	tx *sql.Tx,
	waiting []QueueEntry,
	onMap map[uuid.UUID]bool,
) ([]reselectionCandidate, error) {
	seen := map[uuid.UUID]bool{}
	var out []reselectionCandidate

	for _, entry := range waiting {
		if entry.PartyID == nil || entry.FormingMatchID != nil {
			continue
		}
		if onMap[*entry.PartyID] {
			// Already holding seats on this map. Belt and braces alongside the
			// forming_match_id check above.
			continue
		}
		if seen[*entry.PartyID] {
			continue
		}
		seen[*entry.PartyID] = true

		members, err := listPartyMembersTx(ctx, tx, *entry.PartyID)
		if err != nil {
			return nil, err
		}
		if len(members) == 0 {
			continue
		}

		anyAway := false
		for _, m := range members {
			away, err := userIsAwayTx(ctx, tx, m.UserID)
			if err != nil {
				return nil, err
			}
			if away {
				anyAway = true
				break
			}
		}
		if anyAway {
			continue
		}

		out = append(out, reselectionCandidate{partyID: *entry.PartyID, members: members})
	}

	sort.Slice(out, func(i, j int) bool {
		return out[i].partyID.String() < out[j].partyID.String()
	})
	return out, nil
}

// muByUserTx reads every relevant player's rating in one batch, defaulting the
// unrated to the documented cold-start prior.
func (s *Store) muByUserTx(ctx context.Context, joinCtx *ModeQueueJoinContext, userIDs []uuid.UUID) (map[uuid.UUID]float64, error) {
	ratings, err := s.GetPlayerRatings(ctx, joinCtx.Game.ID, joinCtx.Mode.ModeKey, userIDs)
	if err != nil {
		return nil, err
	}
	out := make(map[uuid.UUID]float64, len(userIDs))
	for _, id := range userIDs {
		if r, ok := ratings[id]; ok {
			out[id] = r.Mu
			continue
		}
		out[id] = rating.UnratedMu
	}
	return out, nil
}

// pairBySeatPath matches a candidate party's members onto the seats a placed
// party is giving up, one member per seat on the same queue path.
//
// Same queue path is the whole of "same shape". It is what makes a swap a
// substitution rather than a re-solve: the seats stay filled by the same paths
// in the same counts, so every gap the placement solver already satisfied is
// still satisfied afterwards. A candidate that cannot be paired exactly is not
// eligible, which is how a party wider or narrower than the vacancy is refused
// rather than partially seated.
func pairBySeatPath(seats []FormingAssignment, members []JoinPartyMemberInput) (map[string]uuid.UUID, bool) {
	if len(seats) != len(members) {
		return nil, false
	}

	byPath := map[string][]string{}
	for _, seat := range seats {
		path := strings.TrimSpace(seat.QueuePath)
		byPath[path] = append(byPath[path], seat.SeatKey)
	}

	seatByUser := make(map[string]uuid.UUID, len(members))
	for _, m := range members {
		path := strings.TrimSpace(m.QueuePath)
		open := byPath[path]
		if len(open) == 0 {
			return nil, false
		}
		seatByUser[open[0]] = m.UserID
		byPath[path] = open[1:]
	}
	return seatByUser, true
}

// applyReselectionTx performs the swap: the outgoing party's seats are vacated
// and immediately refilled by the incoming one, inside the reconcile's
// transaction and under the advisory lock it already holds.
func (s *Store) applyReselectionTx(
	ctx context.Context,
	tx *sql.Tx,
	formingMatchID, outgoingPartyID uuid.UUID,
	incoming *reselectionCandidate,
	seatByUser map[string]uuid.UUID,
) error {
	if _, err := tx.ExecContext(ctx, `
		UPDATE forming_match_assignments
		SET user_id = NULL, party_id = NULL, source = 'solo', table_id = NULL
		WHERE forming_match_id = $1 AND party_id = $2
	`, formingMatchID, outgoingPartyID); err != nil {
		return err
	}
	// Their waiting row is untouched on purpose -- joined_at included. They
	// lose this table, not their place in line, and they were never told a
	// match had formed, so there is nothing to explain to them.
	if _, err := tx.ExecContext(ctx, `
		UPDATE game_queues SET forming_match_id = NULL
		WHERE party_id = $1 AND status = 'waiting'
	`, outgoingPartyID); err != nil {
		return err
	}
	if err := s.markPartyStatusTx(ctx, tx, outgoingPartyID, PartyStatusWaiting); err != nil {
		return err
	}

	source := "party"
	if len(incoming.members) == 1 {
		source = "solo"
	}
	placed := make(map[string]string, len(seatByUser))
	for seatKey, userID := range seatByUser {
		placed[userID.String()] = seatKey
	}
	if err := s.persistFormingSlotsTx(ctx, tx, formingMatchID, incoming.partyID, placed, source, nil); err != nil {
		return err
	}
	if err := s.markPartyStatusTx(ctx, tx, incoming.partyID, PartyStatusPlaced); err != nil {
		return err
	}
	return s.linkPartyQueuesToFormingTx(ctx, tx, incoming.partyID, formingMatchID)
}

func sortedPartyIDs(byParty map[uuid.UUID][]FormingAssignment) []uuid.UUID {
	out := make([]uuid.UUID, 0, len(byParty))
	for id := range byParty {
		out = append(out, id)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].String() < out[j].String() })
	return out
}
