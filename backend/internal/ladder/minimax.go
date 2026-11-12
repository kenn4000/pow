package ladder

import (
	"prisonersofwar/internal/game"
)

const minimaxWinScore = 1000

type minimaxKey struct {
	handA            uint32
	handB            uint32
	chainValue       int8
	chainSuit        int8
	current          PlayerID
	openingResponder PlayerID
	chainIsWild      bool
	openingPending   bool
}

type minimaxEntry struct {
	score int
	bound int8
}

const (
	minimaxExact int8 = iota
	minimaxLower
	minimaxUpper
)

type ladderMinimax struct {
	cpu      PlayerID
	maxDepth int
	table    map[minimaxKey]minimaxEntry
}

func BestMinimaxPlay(g *LadderGame, player PlayerID) (game.Card, bool) {
	if g == nil || g.current != player || g.winner != nil || g.openingPending {
		return game.Card{}, false
	}
	legal := g.LegalPlays(player)
	if len(legal) == 0 {
		return game.Card{}, false
	}
	search := newLadderMinimax(g, player)
	best, bestScore := legal[0], -minimaxWinScore*2
	for _, card := range legal {
		next := Restore(g.State())
		if err := next.Play(player, card); err != nil {
			continue
		}
		score := search.solve(next, -minimaxWinScore*2, minimaxWinScore*2)
		if score > bestScore {
			best, bestScore = card, score
		}
	}
	return best, true
}

func BestMinimaxOpening(g *LadderGame, player PlayerID) (game.Card, bool) {
	if g == nil || g.current != player || g.winner != nil ||
		!g.openingPending || g.openingCard != nil {
		return game.Card{}, false
	}
	state := g.State()
	hand := state.HandA
	if player == PlayerB {
		hand = state.HandB
	}
	if len(hand) == 0 {
		return game.Card{}, false
	}
	search := newLadderMinimax(g, player)
	best, bestScore := hand[0], -minimaxWinScore*2
	for _, card := range hand {
		next := Restore(state)
		if err := next.OpenStack(player, card); err != nil {
			continue
		}
		score := search.solve(next, -minimaxWinScore*2, minimaxWinScore*2)
		if score > bestScore {
			best, bestScore = card, score
		}
	}
	return best, true
}

func newLadderMinimax(g *LadderGame, cpu PlayerID) *ladderMinimax {
	state := g.State()
	return &ladderMinimax{
		cpu: cpu, maxDepth: len(state.HandA) + len(state.HandB),
		table: make(map[minimaxKey]minimaxEntry),
	}
}

func (s *ladderMinimax) solve(g *LadderGame, alpha, beta int) int {
	if winner := g.Winner(); winner != nil {
		remaining := len(g.handA) + len(g.handB)
		depth := s.maxDepth - remaining
		if *winner == s.cpu {
			return minimaxWinScore - depth
		}
		return -minimaxWinScore + depth
	}

	state := g.State()
	if state.OpeningPending && state.OpeningCard != nil {
		next := Restore(state)
		if err := next.PlayOpeningCard(); err == nil {
			return s.solve(next, alpha, beta)
		}
		return -minimaxWinScore * 2
	}

	key := minimaxStateKey(state)
	alphaOriginal, betaOriginal := alpha, beta
	if entry, ok := s.table[key]; ok {
		switch entry.bound {
		case minimaxExact:
			return entry.score
		case minimaxLower:
			if entry.score > alpha {
				alpha = entry.score
			}
		case minimaxUpper:
			if entry.score < beta {
				beta = entry.score
			}
		}
		if alpha >= beta {
			return entry.score
		}
	}

	current := state.Current
	maximizing := current == s.cpu
	best := -minimaxWinScore * 2
	if !maximizing {
		best = minimaxWinScore * 2
	}
	if state.OpeningPending {
		hand := state.HandA
		if current == PlayerB {
			hand = state.HandB
		}
		for _, card := range hand {
			next := Restore(state)
			if err := next.OpenStack(current, card); err != nil {
				continue
			}
			score := s.solve(next, alpha, beta)
			if maximizing && score > best || !maximizing && score < best {
				best = score
			}
			if maximizing {
				if best > alpha {
					alpha = best
				}
			} else if best < beta {
				beta = best
			}
			if alpha >= beta {
				break
			}
		}
	} else {
		legal := g.LegalPlays(current)
		if len(legal) == 0 {
			next := Restore(state)
			if err := next.PassTurn(current); err != nil {
				return -minimaxWinScore * 2
			}
			best = s.solve(next, alpha, beta)
		} else {
			for _, card := range legal {
				next := Restore(state)
				if err := next.Play(current, card); err != nil {
					continue
				}
				score := s.solve(next, alpha, beta)
				if maximizing && score > best || !maximizing && score < best {
					best = score
				}
				if maximizing {
					if best > alpha {
						alpha = best
					}
				} else if best < beta {
					beta = best
				}
				if alpha >= beta {
					break
				}
			}
		}
	}

	bound := minimaxExact
	if best <= alphaOriginal {
		bound = minimaxUpper
	} else if best >= betaOriginal {
		bound = minimaxLower
	}
	s.table[key] = minimaxEntry{score: best, bound: bound}
	return best
}

func minimaxStateKey(state LadderState) minimaxKey {
	chainValue, chainSuit := int8(-1), int8(-1)
	if state.ChainValue != nil {
		chainValue = int8(*state.ChainValue)
	}
	if state.ChainSuit != nil {
		chainSuit = int8(*state.ChainSuit)
	}
	return minimaxKey{
		handA: cardMask(state.HandA), handB: cardMask(state.HandB),
		chainValue: chainValue, chainSuit: chainSuit, current: state.Current,
		openingResponder: state.OpeningResponder,
		chainIsWild:      state.ChainIsWild, openingPending: state.OpeningPending,
	}
}

func cardMask(cards []game.Card) uint32 {
	var mask uint32
	for _, card := range cards {
		if card.Valid() {
			mask |= 1 << (int(card.Suit)*9 + card.Face - 1)
		}
	}
	return mask
}
