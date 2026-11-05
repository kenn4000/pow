package lootbox

import (
	"math/rand"
	"testing"
	"time"

	"prisonersofwar/internal/profile"
)

func TestLoadedDeckAddsOneStartingCard(t *testing.T) {
	now := time.Now()
	p, err := profile.New(now)
	if err != nil {
		t.Fatal(err)
	}
	p.DrawsAvailable = 1
	normal, err := BeginCampDraw(p, rand.New(rand.NewSource(1)))
	if err != nil {
		t.Fatal(err)
	}
	if got := len(normal.StartingHand()); got != 3 {
		t.Fatalf("normal starting hand has %d cards, want 3", got)
	}

	p.SkillNodes["loaded-deck"] = true
	loaded, err := BeginCampDraw(p, rand.New(rand.NewSource(1)))
	if err != nil {
		t.Fatal(err)
	}
	if got := len(loaded.StartingHand()); got != 4 {
		t.Fatalf("Loaded Deck starting hand has %d cards, want 4", got)
	}
}

func TestCampDrawRerollAwardsOnlyTheHigherRankedPrize(t *testing.T) {
	for seed := int64(0); seed < 100; seed++ {
		now := time.Now()
		p, err := profile.New(now)
		if err != nil {
			t.Fatal(err)
		}
		p.DrawsAvailable = 1
		p.SkillNodes["reroll"] = true
		rng := rand.New(rand.NewSource(seed))
		session, err := BeginCampDraw(p, rng)
		if err != nil {
			t.Fatal(err)
		}
		for session.exchangeIndex < 3 {
			if _, err := session.Play(0, rng); err != nil {
				t.Fatal(err)
			}
		}
		result, err := session.Finish(p, rng, nil)
		if err != nil {
			t.Fatal(err)
		}
		if !result.Rerolled {
			continue
		}
		if p.Marks != result.Reward.Marks || p.Crowns != result.Reward.Crowns ||
			p.DrawsAvailable != 0 {
			t.Fatalf("rerolled prize was not awarded exactly once: result=%+v marks=%d crowns=%d draws=%d",
				result, p.Marks, p.Crowns, p.DrawsAvailable)
		}
		if result.Reward.ChromaID != "" && !p.OwnedChromas[result.Reward.ChromaID] {
			t.Fatalf("rerolled chroma %q was not added to the profile", result.Reward.ChromaID)
		}
		return
	}
	t.Fatal("none of the deterministic rolls selected a higher-ranked reroll")
}

func TestHigherRankedCampDrawPrizeOrdering(t *testing.T) {
	tests := []struct {
		name      string
		candidate BoxOpenResult
		current   BoxOpenResult
		want      bool
	}{
		{"rarer skin", BoxOpenResult{ChromaRarity: "rare"}, BoxOpenResult{ChromaRarity: "common", Marks: 1000}, true},
		{"currency value", BoxOpenResult{Marks: 500}, BoxOpenResult{Marks: 100}, true},
		{"lower currency value", BoxOpenResult{Marks: 100}, BoxOpenResult{Marks: 500}, false},
		{"equal value", BoxOpenResult{Marks: 100}, BoxOpenResult{Marks: 100}, false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := higherRankedReward(test.candidate, test.current); got != test.want {
				t.Fatalf("higherRankedReward() = %t, want %t", got, test.want)
			}
		})
	}
}
