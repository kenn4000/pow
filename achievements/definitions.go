// Package achievements catalogs and evaluates lifetime progression.
package achievements

// Tuning targets: approximately 66,000 Marks, 1,100 Crowns, 15 Skill Tree
// Points, 1 Chroma Pick token, and at least 8 distinct badges.

const (
	BountyConnoisseurRewardI   int64 = 200
	BountyConnoisseurRewardII  int64 = 250
	BountyConnoisseurRewardIII int64 = 300
	BountyConnoisseurRewardIV  int64 = 350
	BountyConnoisseurRewardV   int64 = 400
	BountyConnoisseurRewardVI  int64 = 500
	BountyConnoisseurRewardVII int64 = 600

	FavoredAficionadoRewardI   int64 = 400
	FavoredAficionadoRewardII  int64 = 400
	FavoredAficionadoRewardIII int64 = 500
	FavoredAficionadoRewardIV  int64 = 600

	ShiftWhispererRewardI   int64 = 400
	ShiftWhispererRewardII  int64 = 400
	ShiftWhispererRewardIII int64 = 500
	ShiftWhispererRewardIV  int64 = 600

	HeroVarietyRewardI   int64 = 500
	HeroVarietyRewardII  int64 = 2000
	HeroVarietyRewardIII int64 = 5000
)

type Category int

const (
	CatOnboarding Category = iota
	CatStrategy
	CatSkill
	CatComeback
	CatDedication
	CatCollection
	CatRetention
	CatSeatMastery
	CatStreak
	CatBounty
	CatTies
	CatMisc
)

type CounterType int

const (
	CounterNone CounterType = iota
	CounterStreak
	CounterCumulative
	CounterSingleMatch
	CounterBestRecord
)

type Achievement struct {
	ID               string
	Name             string
	Description      string
	Category         Category
	SetID            string
	Hidden           bool
	CounterType      CounterType
	RewardMarks      int64
	RewardCrowns     int64
	RewardChromaPick int
	RewardBadge      string
	RewardHeroBox    int
	IndependentTiers bool
	Tiers            []Tier
	CounterID        string
}

type Tier struct {
	Level            int
	Goal             int
	Name             string
	RewardMarks      int64
	RewardCrowns     int64
	RewardBadge      string
	RewardChromaPick int
	RewardHeroBox    int
}

type Set struct {
	ID      string
	Name    string
	Members []string
}

var achievements = []Achievement{
	{
		ID: "sparse", Name: "Sparse", Description: "Win with carefully limited suit representation in the jails.",
		Category: CatStrategy, SetID: "sparse", CounterType: CounterSingleMatch, CounterID: "sparse",
		Tiers: []Tier{
			{Level: 1, Goal: 2, Name: "I", RewardMarks: 500},
			{Level: 2, Goal: 2, Name: "II", RewardMarks: 1000},
			{Level: 3, Goal: 1, Name: "III", RewardMarks: 1500},
			{Level: 4, Goal: 2, Name: "IV", RewardMarks: 2500, RewardCrowns: 50},
		},
	},
	{
		ID: "efficient", Name: "Efficient", Description: "Win while capturing as many prisoners as possible.",
		Category: CatStrategy, SetID: "efficient", CounterType: CounterSingleMatch, CounterID: "efficient",
		Tiers: []Tier{
			{Level: 1, Goal: 7, Name: "I", RewardMarks: 500},
			{Level: 2, Goal: 8, Name: "II", RewardMarks: 1000},
			{Level: 3, Goal: 9, Name: "III", RewardMarks: 2500, RewardCrowns: 50},
		},
	},
	{
		ID: "bounty_hunter", Name: "Bounty Hunter", Description: "Win with at least three prisoners of the bounty's suit.",
		Category: CatBounty, SetID: "bounty", CounterType: CounterNone, RewardMarks: 1000,
	},
	{
		ID: "bounty_denied", Name: "Bounty Denied", Description: "Win with no prisoners of the bounty's suit.",
		Category: CatBounty, SetID: "bounty", CounterType: CounterNone, RewardMarks: 1500,
	},
	{
		ID: "bounty_decided", Name: "Bounty Decided", Description: "Win because the bounty bonus alone moved your final score ahead.",
		Category: CatBounty, SetID: "bounty", CounterType: CounterNone, RewardMarks: 2500, RewardCrowns: 50,
	},
	{
		ID: "aggressor", Name: "Aggressor", Description: "Win games while assigned to Seat X.",
		Category: CatSeatMastery, SetID: "aggressor", CounterType: CounterCumulative, CounterID: "wins_as_seat_x",
		Tiers: []Tier{
			{Level: 1, Goal: 1, Name: "I", RewardMarks: 200},
			{Level: 2, Goal: 10, Name: "II", RewardMarks: 1000},
			{Level: 3, Goal: 100, Name: "III", RewardMarks: 5000, RewardCrowns: 100, RewardBadge: "badge_aggressor", RewardHeroBox: 1},
		},
	},
	{
		ID: "defender", Name: "Defender", Description: "Win games while assigned to Seat Y.",
		Category: CatSeatMastery, SetID: "defender", CounterType: CounterCumulative, CounterID: "wins_as_seat_y",
		Tiers: []Tier{
			{Level: 1, Goal: 1, Name: "I", RewardMarks: 200},
			{Level: 2, Goal: 10, Name: "II", RewardMarks: 1000},
			{Level: 3, Goal: 100, Name: "III", RewardMarks: 5000, RewardCrowns: 100, RewardBadge: "badge_defender", RewardHeroBox: 1},
		},
	},
	{
		ID: "streak_easy", Name: "Easy Streak", Description: "Win three matches in a row against the Easy CPU.",
		Category: CatStreak, SetID: "win_streaks", CounterType: CounterStreak, CounterID: "streak_easy", RewardMarks: 250,
	},
	{
		ID: "streak_medium", Name: "Medium Streak", Description: "Win three matches in a row against the Normal CPU.",
		Category: CatStreak, SetID: "win_streaks", CounterType: CounterStreak, CounterID: "streak_medium", RewardMarks: 500,
	},
	{
		ID: "streak_hard", Name: "Hard Streak", Description: "Win three matches in a row against the Hard CPU.",
		Category: CatStreak, SetID: "win_streaks", CounterType: CounterStreak, CounterID: "streak_hard", RewardMarks: 1000,
	},
	{
		ID: "streak_ranked", Name: "Ranked Streak", Description: "Win three Ranked matches in a row.",
		Category: CatStreak, SetID: "win_streaks", CounterType: CounterStreak, CounterID: "streak_ranked", RewardMarks: 1500, RewardCrowns: 50,
	},
	{
		ID: "comeback", Name: "Comeback", Description: "Win games after trailing at the end of Round 2.",
		Category: CatComeback, SetID: "comeback", CounterType: CounterCumulative, CounterID: "comeback_wins",
		Tiers: []Tier{
			{Level: 1, Goal: 1, Name: "I", RewardMarks: 500},
			{Level: 2, Goal: 10, Name: "II", RewardMarks: 1500},
			{Level: 3, Goal: 50, Name: "III", RewardMarks: 4000, RewardCrowns: 50, RewardBadge: "badge_resilient", RewardHeroBox: 1},
		},
	},
	{
		ID: "first_clash", Name: "First Clash", Description: "Tie any exchange.",
		Category: CatTies, SetID: "ties", CounterType: CounterCumulative, CounterID: "total_clashes", RewardMarks: 100,
	},
	{
		ID: "clash_master", Name: "Clash Master", Description: "Tie at least three exchanges in one game.",
		Category: CatTies, SetID: "ties", CounterType: CounterBestRecord, CounterID: "max_clashes_in_game", RewardMarks: 500,
	},
	{
		ID: "dead_heat", Name: "Dead Heat", Description: "Finish a match with equal final scores.",
		Category: CatTies, SetID: "ties", CounterType: CounterCumulative, CounterID: "total_match_ties", RewardMarks: 1500, RewardCrowns: 50,
	},
	{
		ID: "collector", Name: "Collector", Description: "Purchase chromas across increasing rarities.",
		Category: CatCollection, SetID: "collector", CounterType: CounterCumulative, CounterID: "chromas_purchased",
		Tiers: []Tier{
			{Level: 1, Goal: 3, Name: "I", RewardMarks: 500},
			{Level: 2, Goal: 3, Name: "II", RewardMarks: 1500},
			{Level: 3, Goal: 3, Name: "III", RewardChromaPick: 1},
		},
	},
	{
		ID: "long_march", Name: "Long March", Description: "Play 100 matches.",
		Category: CatDedication, SetID: "dedication", CounterType: CounterCumulative, CounterID: "total_games", RewardMarks: 2000, RewardHeroBox: 1,
	},
	{
		ID: "hard_way", Name: "The Hard Way", Description: "Beat the Hard CPU 10 times.",
		Category: CatDedication, SetID: "dedication", CounterType: CounterCumulative, CounterID: "hard_cpu_wins",
		RewardMarks: 2500, RewardCrowns: 100, RewardBadge: "badge_determined", RewardHeroBox: 1,
	},
	{
		ID: "login_streak", Name: "Login Streak", Description: "Return on consecutive days.",
		Category: CatRetention, SetID: "login_streak", CounterType: CounterBestRecord, CounterID: "login_streak_best",
		Tiers: []Tier{
			{Level: 1, Goal: 1, Name: "I", RewardMarks: 100},
			{Level: 2, Goal: 7, Name: "II", RewardMarks: 750},
			{Level: 3, Goal: 30, Name: "III", RewardMarks: 3000, RewardCrowns: 50, RewardBadge: "badge_devoted", RewardHeroBox: 1},
		},
	},
	{
		ID: "first_blood", Name: "First Blood", Description: "Win your first match.",
		Category: CatOnboarding, CounterType: CounterNone, RewardMarks: 200,
	},
	{
		ID: "both_sides", Name: "Both Sides", Description: "Win a match as Seat X and a match as Seat Y.",
		Category: CatSeatMastery, CounterType: CounterNone, RewardMarks: 300,
	},
	{
		ID: "the_cycle", Name: "The Cycle", Description: "Win a game with a lane won by suit doubling.",
		Category: CatSkill, CounterType: CounterNone, RewardMarks: 200,
	},
	{
		ID: "master_both", Name: "Master Both", Description: "Win three matches in a row as Seat X and three in a row as Seat Y.",
		Category: CatSeatMastery, CounterType: CounterNone, RewardMarks: 1500,
	},
	{
		ID: "photo_finish", Name: "Photo Finish", Description: "Win a match by exactly one point.",
		Category: CatMisc, CounterType: CounterNone, RewardMarks: 1500, RewardCrowns: 50,
	},
	{
		ID: "no_not_like_that", Name: "No, Not Like That", Description: "Lose a game with at least nine points from one suit.",
		Category: CatMisc, CounterType: CounterNone, RewardMarks: 1000, RewardCrowns: 50,
	},
	{
		ID: "bounty_connoisseur", Name: "Bounty Connoisseur",
		Description: "Win a match with the exact active Bounty modifier shown for each tier.",
		Category:    CatStrategy, SetID: "bounty_connoisseur", CounterType: CounterNone, IndependentTiers: true,
		Tiers: []Tier{
			{Level: 1, Goal: -3, Name: "I", RewardMarks: BountyConnoisseurRewardI},
			{Level: 2, Goal: -2, Name: "II", RewardMarks: BountyConnoisseurRewardII},
			{Level: 3, Goal: -1, Name: "III", RewardMarks: BountyConnoisseurRewardIII},
			{Level: 4, Goal: 0, Name: "IV", RewardMarks: BountyConnoisseurRewardIV},
			{Level: 5, Goal: 1, Name: "V", RewardMarks: BountyConnoisseurRewardV},
			{Level: 6, Goal: 2, Name: "VI", RewardMarks: BountyConnoisseurRewardVI},
			{Level: 7, Goal: 3, Name: "VII", RewardMarks: BountyConnoisseurRewardVII},
		},
	},
	{
		ID: "favored_aficionado", Name: "Favored Aficionado",
		Description: "Win a match with the exact active Favored modifier shown for each tier.",
		Category:    CatStrategy, SetID: "favored_aficionado", CounterType: CounterNone, IndependentTiers: true,
		Tiers: []Tier{
			{Level: 1, Goal: -1, Name: "I", RewardMarks: FavoredAficionadoRewardI},
			{Level: 2, Goal: 0, Name: "II", RewardMarks: FavoredAficionadoRewardII},
			{Level: 3, Goal: 1, Name: "III", RewardMarks: FavoredAficionadoRewardIII},
			{Level: 4, Goal: 2, Name: "IV", RewardMarks: FavoredAficionadoRewardIV},
		},
	},
	{
		ID: "shift_whisperer", Name: "Shift Whisperer",
		Description: "Win a match with the exact active Shift modifier shown for each tier.",
		Category:    CatStrategy, SetID: "shift_whisperer", CounterType: CounterNone, IndependentTiers: true,
		Tiers: []Tier{
			{Level: 1, Goal: 0, Name: "I", RewardMarks: ShiftWhispererRewardI},
			{Level: 2, Goal: 1, Name: "II", RewardMarks: ShiftWhispererRewardII},
			{Level: 3, Goal: 2, Name: "III", RewardMarks: ShiftWhispererRewardIII},
			{Level: 4, Goal: 3, Name: "IV", RewardMarks: ShiftWhispererRewardIV},
		},
	},
	{
		ID: "hero_variety", Name: "Hero Variety",
		Description: "Unlock distinct heroes; The Recruit does not count toward these collection tiers.",
		Category:    CatCollection, SetID: "hero_variety", CounterType: CounterNone,
		Tiers: []Tier{
			{Level: 1, Goal: 3, Name: "Novice", RewardMarks: HeroVarietyRewardI},
			{Level: 2, Goal: 6, Name: "Collector", RewardMarks: HeroVarietyRewardII},
			{Level: 3, Goal: 10, Name: "Completionist", RewardMarks: HeroVarietyRewardIII, RewardCrowns: 500, RewardBadge: "badge_completionist", RewardHeroBox: 1},
		},
	},
}

var sets = []Set{
	{ID: "sparse", Name: "Sparse", Members: []string{"sparse"}},
	{ID: "efficient", Name: "Efficient", Members: []string{"efficient"}},
	{ID: "bounty", Name: "Bounty", Members: []string{"bounty_hunter", "bounty_denied", "bounty_decided"}},
	{ID: "aggressor", Name: "Aggressor", Members: []string{"aggressor"}},
	{ID: "defender", Name: "Defender", Members: []string{"defender"}},
	{ID: "win_streaks", Name: "Win Streaks", Members: []string{"streak_easy", "streak_medium", "streak_hard", "streak_ranked"}},
	{ID: "comeback", Name: "Comeback", Members: []string{"comeback"}},
	{ID: "ties", Name: "Ties", Members: []string{"first_clash", "clash_master", "dead_heat"}},
	{ID: "collector", Name: "Collector", Members: []string{"collector"}},
	{ID: "dedication", Name: "Dedication", Members: []string{"long_march", "hard_way"}},
	{ID: "login_streak", Name: "Login Streak", Members: []string{"login_streak"}},
	{ID: "bounty_connoisseur", Name: "Bounty Connoisseur", Members: []string{"bounty_connoisseur"}},
	{ID: "favored_aficionado", Name: "Favored Aficionado", Members: []string{"favored_aficionado"}},
	{ID: "shift_whisperer", Name: "Shift Whisperer", Members: []string{"shift_whisperer"}},
	{ID: "hero_variety", Name: "Hero Variety", Members: []string{"hero_variety"}},
}

func All() []Achievement {
	result := make([]Achievement, len(achievements))
	for i, achievement := range achievements {
		result[i] = achievement
		result[i].Tiers = append([]Tier(nil), achievement.Tiers...)
	}
	return result
}

func Find(id string) (Achievement, bool) {
	for _, achievement := range achievements {
		if achievement.ID == id {
			achievement.Tiers = append([]Tier(nil), achievement.Tiers...)
			return achievement, true
		}
	}
	return Achievement{}, false
}

func AllSets() []Set {
	result := make([]Set, len(sets))
	for i, set := range sets {
		result[i] = set
		result[i].Members = append([]string(nil), set.Members...)
	}
	return result
}
