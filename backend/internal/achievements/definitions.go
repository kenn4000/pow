// Package achievements catalogs and evaluates lifetime progression.
package achievements

type Category int

const (
	CatCampaign Category = iota
	CatFeat
	CatPerMode
	CatMeta
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
	ID                  string
	Name                string
	Description         string
	Category            Category
	SetID               string
	Hidden              bool
	CounterType         CounterType
	CounterID           string
	ProgressUnit        string
	Goal                int
	CampaignIDs         []string
	ModeID              string
	RewardMarks         int64
	RewardCrowns        int64
	RewardSkillPoints   int
	RewardChromaPick    int
	RewardBadge         string
	RewardHeroBox       int
	RewardWoodenBoxes   int
	RewardSilverBoxes   int
	RewardGoldBoxes     int
	RewardUndoTokens    int
	RewardRestartTokens int
	RewardSkipTokens    int
	RewardSwapTokens    int
	IndependentTiers    bool
	Tiers               []Tier
}

type Tier struct {
	Level               int
	Goal                int
	Name                string
	RewardMarks         int64
	RewardCrowns        int64
	RewardSkillPoints   int
	RewardBadge         string
	RewardChromaPick    int
	RewardHeroBox       int
	RewardWoodenBoxes   int
	RewardSilverBoxes   int
	RewardGoldBoxes     int
	RewardUndoTokens    int
	RewardRestartTokens int
	RewardSkipTokens    int
	RewardSwapTokens    int
}

type Set struct {
	ID      string
	Name    string
	Members []string
}

var achievements = buildAchievements()

func buildAchievements() []Achievement {
	result := []Achievement{
		{ID: "camp_pow", Name: "Prisoners of War Campaign", Description: "Complete every Basic campaign mission.", Category: CatCampaign, SetID: "campaigns", CampaignIDs: []string{"basic"}, RewardMarks: 3000, RewardCrowns: 10},
		{ID: "camp_abandon", Name: "Abandon Campaign", Description: "Complete every Abandon campaign mission.", Category: CatCampaign, SetID: "campaigns", CampaignIDs: []string{"abandon"}, RewardMarks: 3000, RewardCrowns: 10},
		{ID: "camp_ladder", Name: "Ladder Campaign", Description: "Complete every Ladder campaign mission.", Category: CatCampaign, SetID: "campaigns", CampaignIDs: []string{"ladder"}, RewardMarks: 3000, RewardCrowns: 10},
		{ID: "camp_poker", Name: "Poker Campaign", Description: "Complete every Poker campaign mission.", Category: CatCampaign, SetID: "campaigns", CampaignIDs: []string{"poker"}, RewardMarks: 3000, RewardCrowns: 10},
		{ID: "camp_rummy", Name: "Rummy Campaign", Description: "Complete every Rummy campaign mission.", Category: CatCampaign, SetID: "campaigns", CampaignIDs: []string{"rummy"}, RewardMarks: 3000, RewardCrowns: 10},
		{ID: "camp_puzzler", Name: "Puzzler Campaign", Description: "Complete every Puzzler campaign mission.", Category: CatCampaign, SetID: "campaigns", CampaignIDs: []string{"puzzler"}, RewardMarks: 3000, RewardCrowns: 10},
		{ID: "camp_tug_war", Name: "Tug War Campaign", Description: "Complete every Tug War campaign mission.", Category: CatCampaign, SetID: "campaigns", CampaignIDs: []string{"tug-war"}, RewardMarks: 3000, RewardCrowns: 10},

		{ID: "sparse", Name: "Sparse", Description: "Win with exactly two suits represented in your jail.", Category: CatFeat, SetID: "other-feats", CounterType: CounterCumulative, CounterID: "feat_sparse", ProgressUnit: "qualifying wins", Goal: 1, RewardSkillPoints: 1},
		{ID: "efficient", Name: "Efficient", Description: "Win with all 9 prisoners and no clashes.", Category: CatFeat, SetID: "other-feats", CounterType: CounterCumulative, CounterID: "feat_efficient", ProgressUnit: "qualifying wins", Goal: 1, RewardSkillPoints: 1},
		{ID: "bounty_hunter", Name: "Bounty Hunter", Description: "Win with at least 3 prisoners matching the bounty role.", Category: CatFeat, SetID: "bounty-feats", CounterType: CounterCumulative, CounterID: "feat_bounty_hunter", ProgressUnit: "qualifying wins", Goal: 1, RewardSkillPoints: 1},
		{ID: "bounty_denied", Name: "Bounty Denied", Description: "Win with no prisoners matching the bounty role.", Category: CatFeat, SetID: "bounty-feats", CounterType: CounterCumulative, CounterID: "feat_bounty_denied", ProgressUnit: "qualifying wins", Goal: 1, RewardSkillPoints: 1},
		{ID: "bounty_decided", Name: "Bounty Decided", Description: "Win when your non-bounty score does not beat the opponent, but your final score does.", Category: CatFeat, SetID: "bounty-feats", CounterType: CounterCumulative, CounterID: "feat_bounty_decided", ProgressUnit: "qualifying wins", Goal: 1, RewardSkillPoints: 1},
		{ID: "comeback", Name: "Comeback", Description: "Win a game after trailing at the end of Round 2.", Category: CatFeat, SetID: "other-feats", CounterType: CounterCumulative, CounterID: "comeback_wins", ProgressUnit: "comeback wins", Goal: 1, RewardSkillPoints: 1},
		{ID: "clash_master", Name: "Clash Master", Description: "Tie at least 3 exchanges in one game.", Category: CatFeat, SetID: "other-feats", CounterType: CounterCumulative, CounterID: "max_clashes_in_game", ProgressUnit: "clashes in one game", Goal: 3, RewardSkillPoints: 1},
		{ID: "dead_heat", Name: "Dead Heat", Description: "Finish a match with equal final scores.", Category: CatFeat, SetID: "other-feats", CounterType: CounterCumulative, CounterID: "total_match_ties", ProgressUnit: "tied matches", Goal: 1, RewardSkillPoints: 1},
		{ID: "photo_finish", Name: "Photo Finish", Description: "Win a match by exactly 1 point.", Category: CatFeat, SetID: "other-feats", CounterType: CounterCumulative, CounterID: "feat_photo_finish", ProgressUnit: "qualifying wins", Goal: 1, RewardSkillPoints: 1},
	}

	modes := []struct {
		id, name, box, token string
	}{
		{"main", "Prisoners of War", "silver", "undo"},
		{"fog", "Fog of War", "silver", "undo"},
		{"abandon", "Abandon", "wooden", "restart"},
		{"poker", "Poker", "silver", "restart"},
		{"rummy", "Rummy", "wooden", "restart"},
		{"puzzler", "Puzzler", "wooden", "swap"},
		{"ladder", "Ladder", "wooden", "skip"},
		{"tug-war", "Tug War", "wooden", "skip"},
	}
	for _, mode := range modes {
		result = append(result, Achievement{
			ID: "mode_" + mode.id, Name: mode.name, Description: "Win this game type 1, 10, and 30 times.",
			Category: CatPerMode, SetID: "per-mode", CounterType: CounterCumulative,
			CounterID: "mode_wins_" + mode.id, ProgressUnit: "wins", ModeID: mode.id,
			Tiers: standardTiers(mode.box, mode.token, []int{1, 10, 30}),
		})
	}

	meta := []Achievement{
		{ID: "chroma_collector", Name: "Chroma Collector", Description: "Own 5, 20, and 50 chromas beyond the three starter chromas.", CounterType: CounterBestRecord, CounterID: "chroma_count", ProgressUnit: "chromas owned", Tiers: standardTiers("gold", "swap", []int{5, 20, 50})},
		{ID: "lucky_draw", Name: "Lucky Draw", Description: "Open 5, 20, and 50 boxes.", CounterType: CounterCumulative, CounterID: "boxes_opened", ProgressUnit: "boxes opened", Tiers: standardTiers("gold", "skip", []int{5, 20, 50})},
		{ID: "win_streak_humans", Name: "Win Streak (Humans)", Description: "Win 3, 6, and 12 consecutive matches against human opponents.", CounterType: CounterBestRecord, CounterID: "human_win_streak_best", ProgressUnit: "consecutive human wins", Tiers: standardTiers("gold", "undo", []int{3, 6, 12})},
		{ID: "login_streak", Name: "Login Streak", Description: "Log in on 1, 10, and 30 consecutive days.", CounterType: CounterBestRecord, CounterID: "login_streak_best", ProgressUnit: "consecutive days", Tiers: standardTiers("gold", "restart", []int{1, 10, 30})},
		{ID: "long_march", Name: "Long March", Description: "Win 20, 50, and 100 matches.", CounterType: CounterCumulative, CounterID: "total_wins", ProgressUnit: "wins", Tiers: standardTiers("gold", "undo", []int{20, 50, 100})},
		{ID: "hero_variety", Name: "Hero Variety", Description: "Win at least 10 matches while winning with 3, 6, and 10 different heroes.", CounterType: CounterBestRecord, CounterID: "hero_variety", ProgressUnit: "heroes with wins", Tiers: standardTiers("gold", "swap", []int{3, 6, 10})},
		{ID: "branch_master", Name: "Branch Master", Description: "Fully unlock 1, 3, and all 6 Skill Tree branches.", CounterType: CounterBestRecord, CounterID: "branches_unlocked", ProgressUnit: "fully unlocked branches", Tiers: standardTiers("gold", "skip", []int{1, 3, 6})},
		{ID: "tinkerer", Name: "Tinkerer", Description: "Use 10, 50, and 200 consumables.", CounterType: CounterCumulative, CounterID: "consumables_used", ProgressUnit: "consumables used", Tiers: standardTiers("gold", "restart", []int{10, 50, 200})},
		{ID: "marathon", Name: "Marathon", Description: "Play 10, 30, and 60 matches in a single week.", CounterType: CounterBestRecord, CounterID: "weekly_matches_best", ProgressUnit: "matches in one week", Tiers: standardTiers("gold", "undo", []int{10, 30, 60})},
		{ID: "mode_tourist", Name: "Mode Tourist", Description: "Win in 2, 4, and 7 different game types.", CounterType: CounterBestRecord, CounterID: "mode_tourist", ProgressUnit: "game types won", Tiers: standardTiers("gold", "swap", []int{2, 4, 7})},
		{ID: "ranked_regular", Name: "Ranked Regular", Description: "Play 10, 50, and 200 ranked matches.", CounterType: CounterCumulative, CounterID: "ranked_matches_played", ProgressUnit: "ranked matches", Tiers: standardTiers("gold", "skip", []int{10, 50, 200})},
	}
	for i := range meta {
		meta[i].Category = CatMeta
		meta[i].SetID = "meta"
	}
	result = append(result, meta...)
	return result
}

func standardTiers(box, token string, goals []int) []Tier {
	tiers := make([]Tier, len(goals))
	for i, goal := range goals {
		tiers[i] = Tier{Level: i + 1, Goal: goal, Name: []string{"I", "II", "III"}[i], RewardMarks: int64((i + 1) * 400)}
		if i >= 1 {
			tiers[i].RewardCrowns = int64(i)
		}
	}
	last := &tiers[len(tiers)-1]
	switch box {
	case "wooden":
		last.RewardWoodenBoxes = 1
	case "silver":
		last.RewardSilverBoxes = 1
	case "gold":
		last.RewardGoldBoxes = 1
	}
	switch token {
	case "undo":
		last.RewardUndoTokens = 1
	case "restart":
		last.RewardRestartTokens = 1
	case "skip":
		last.RewardSkipTokens = 1
	case "swap":
		last.RewardSwapTokens = 1
	}
	return tiers
}

var sets = []Set{
	{ID: "campaigns", Name: "Campaigns", Members: []string{"camp_pow", "camp_abandon", "camp_ladder", "camp_poker", "camp_rummy", "camp_puzzler", "camp_tug_war"}},
	{ID: "bounty-feats", Name: "Bounty Feats", Members: []string{"bounty_hunter", "bounty_denied", "bounty_decided"}},
	{ID: "other-feats", Name: "Other Feats", Members: []string{"sparse", "efficient", "comeback", "clash_master", "dead_heat", "photo_finish"}},
	{ID: "per-mode", Name: "Per-Mode", Members: []string{"mode_main", "mode_fog", "mode_abandon", "mode_poker", "mode_rummy", "mode_puzzler", "mode_ladder", "mode_tug-war"}},
	{ID: "meta", Name: "Meta", Members: []string{"chroma_collector", "lucky_draw", "win_streak_humans", "login_streak", "long_march", "hero_variety", "branch_master", "tinkerer", "marathon", "mode_tourist", "ranked_regular"}},
}

func All() []Achievement {
	result := make([]Achievement, len(achievements))
	for i, achievement := range achievements {
		result[i] = achievement
		result[i].CampaignIDs = append([]string(nil), achievement.CampaignIDs...)
		result[i].Tiers = append([]Tier(nil), achievement.Tiers...)
	}
	return result
}

func Find(id string) (Achievement, bool) {
	for _, achievement := range achievements {
		if achievement.ID == id {
			achievement.CampaignIDs = append([]string(nil), achievement.CampaignIDs...)
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
