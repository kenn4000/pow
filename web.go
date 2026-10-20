package main

import (
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"time"

	"prisonersofwar/ai"
	"prisonersofwar/game"
)

//go:embed web/index.html
var webFiles embed.FS

type webServer struct {
	mu          sync.Mutex
	rng         *rand.Rand
	defaultMode string
	session     *webSession
}

type webSession struct {
	state         game.GameState
	mode          string
	whitePlayer   int
	phase         string
	actor         int
	handoffTarget int
	handoffPhase  string
	exchange      int
	attacker      int
	attackIndex   int
	attackCard    game.Card
	lastAttack    game.Card
	lastDefense   game.Card
	lastOutcome   game.Outcome
	hasLastBattle bool
	slots         [9]jailCell
	message       string
	winner        int
	tiebreak      string
}

type jailCell struct {
	Card       *game.Card `json:"card,omitempty"`
	CapturedBy string     `json:"capturedBy,omitempty"`
	Clash      bool       `json:"clash,omitempty"`
}

type webCard struct {
	Card       game.Card `json:"card"`
	Index      int       `json:"index"`
	Selectable bool      `json:"selectable"`
}

type webPlayer struct {
	Color         string    `json:"color"`
	You           bool      `json:"you"`
	Seat          string    `json:"seat"`
	Score         int       `json:"score"`
	Hand          []webCard `json:"hand"`
	HiddenHand    int       `json:"hiddenHand"`
	Pool          []webCard `json:"pool"`
	HiddenPool    int       `json:"hiddenPool"`
	CanSelectHand bool      `json:"canSelectHand"`
	CanSelectPool bool      `json:"canSelectPool"`
}

type webView struct {
	Started      bool        `json:"started"`
	Mode         string      `json:"mode,omitempty"`
	Phase        string      `json:"phase,omitempty"`
	Round        int         `json:"round,omitempty"`
	Exchange     int         `json:"exchange,omitempty"`
	Actor        int         `json:"actor"`
	Attacker     int         `json:"attacker"`
	Bounty       game.Card   `json:"bounty"`
	Players      []webPlayer `json:"players,omitempty"`
	JailSlots    []jailCell  `json:"jailSlots,omitempty"`
	BattleAttack *game.Card  `json:"battleAttack,omitempty"`
	BattleDefend *game.Card  `json:"battleDefend,omitempty"`
	Message      string      `json:"message,omitempty"`
	ScoreLine    string      `json:"scoreLine,omitempty"`
	CanContinue  bool        `json:"canContinue"`
	Winner       int         `json:"winner"`
	Tiebreak     string      `json:"tiebreak,omitempty"`
}

type actionRequest struct {
	Action string `json:"action"`
	Mode   string `json:"mode"`
	Index  int    `json:"index"`
	Cards  []int  `json:"cards"`
	Seat   string `json:"seat"`
}

func startWebUI(listenAddress, defaultMode string) error {
	listener, err := net.Listen("tcp", listenAddress)
	if err != nil {
		return fmt.Errorf("start local game server: %w", err)
	}
	server := &webServer{
		rng:         rand.New(rand.NewSource(time.Now().UnixNano())),
		defaultMode: defaultMode,
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/", server.handlePage)
	mux.HandleFunc("/api/state", server.handleState)
	mux.HandleFunc("/api/new", server.handleNewGame)
	mux.HandleFunc("/api/action", server.handleAction)
	address := "http://" + listener.Addr().String()
	fmt.Printf("Prisoners of War UI running at %s\n", address)
	openLocalBrowser(address)
	return http.Serve(listener, mux)
}

func openLocalBrowser(address string) {
	var command *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		command = exec.Command("rundll32", "url.dll,FileProtocolHandler", address)
	case "darwin":
		command = exec.Command("open", address)
	default:
		command = exec.Command("xdg-open", address)
	}
	if err := command.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "Could not open a browser automatically: %v\nOpen %s manually.\n", err, address)
	}
}

func (s *webServer) handlePage(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	page, err := webFiles.ReadFile("web/index.html")
	if err != nil {
		http.Error(w, "UI asset unavailable", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	if _, err := w.Write(page); err != nil {
		fmt.Fprintf(os.Stderr, "write game UI response: %v\n", err)
	}
}

func (s *webServer) handleState(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	writeJSON(w, http.StatusOK, s.view())
}

func (s *webServer) handleNewGame(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var request actionRequest
	if err := decodeJSON(r, &request); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	mode := request.Mode
	if mode == "" {
		mode = s.defaultMode
	}
	if !validWebMode(mode) {
		writeError(w, http.StatusBadRequest, errors.New("mode must be hvh, easy, normal, or hard"))
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	state, err := game.NewGame(s.rng)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	whitePlayer, err := state.PlayerAtSeat(game.SeatX)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	s.session = &webSession{
		state:       state,
		mode:        mode,
		whitePlayer: whitePlayer,
		phase:       "commit",
		actor:       0,
		winner:      -1,
		message:     "Choose three cards from your hand to commit for Round 1.",
	}
	if err := s.advanceAI(); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, s.view())
}

func (s *webServer) handleAction(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var request actionRequest
	if err := decodeJSON(r, &request); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.session == nil {
		writeError(w, http.StatusConflict, errors.New("start a game first"))
		return
	}
	if err := s.applyAction(request); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if err := s.advanceAI(); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, s.view())
}

func (s *webServer) applyAction(request actionRequest) error {
	session := s.session
	switch request.Action {
	case "commit":
		if session.phase != "commit" || isAI(session.mode, session.actor) {
			return errors.New("it is not your commit turn")
		}
		if err := game.Commit(&session.state, session.actor, request.Cards); err != nil {
			return err
		}
		if session.mode == "hvh" {
			if session.actor == 0 {
				session.actor = 1
				session.handoffTarget = 1
				session.handoffPhase = "commit"
				session.phase = "handoff"
				session.message = "Pass the screen to Player 2, then continue."
				return nil
			}
			s.beginBattles()
			return nil
		}
		session.actor = 1
		session.message = "Player 2 is committing."
	case "ready":
		if session.phase != "handoff" {
			return errors.New("there is no handoff to acknowledge")
		}
		session.actor = session.handoffTarget
		session.phase = session.handoffPhase
		session.handoffPhase = ""
		if session.phase == "commit" {
			session.message = fmt.Sprintf("Player %d, choose three cards to commit.", session.actor+1)
		} else if session.phase == "attack" {
			session.message = fmt.Sprintf("Player %d attacks. Choose a card from their pile.", session.actor+1)
		} else {
			session.message = "Choose a response to the revealed attack."
		}
	case "attack":
		if session.phase != "attack" || isAI(session.mode, session.attacker) {
			return errors.New("it is not your attack turn")
		}
		if err := s.selectAttack(request.Index); err != nil {
			return err
		}
	case "defend":
		if session.phase != "defend" || isAI(session.mode, session.actor) {
			return errors.New("it is not your defense turn")
		}
		if err := s.resolveBattle(request.Index); err != nil {
			return err
		}
	case "continue":
		if session.phase != "result" {
			return errors.New("there is no battle result to continue from")
		}
		if err := s.continueAfterBattle(); err != nil {
			return err
		}
	case "seat":
		if session.phase != "seat" {
			return errors.New("Round 3 seat selection is no longer active")
		}
		if isAI(session.mode, session.actor) {
			return errors.New("the AI chooses the Round 3 seat")
		}
		seat := game.SeatX
		if strings.EqualFold(request.Seat, "Y") {
			seat = game.SeatY
		} else if !strings.EqualFold(request.Seat, "X") {
			return errors.New("choose seat X or Y")
		}
		s.assignSeat(session.actor, seat)
		s.beginRoundThree()
	default:
		return errors.New("unknown action")
	}
	return nil
}

func (s *webServer) advanceAI() error {
	session := s.session
	for session != nil && isAI(session.mode, session.actor) {
		switch session.phase {
		case "commit":
			indices, err := ai.ChooseCommit(session.state, session.actor, webDifficulty(session.mode), s.rng)
			if err != nil {
				return err
			}
			if err := game.Commit(&session.state, session.actor, indices); err != nil {
				return err
			}
			session.message = "The opponent has committed."
			if session.actor == 0 {
				session.actor = 1
			} else {
				s.beginBattles()
			}
		case "attack":
			index, err := ai.ChoosePoolIndex(session.state, session.attacker, true, nil, webDifficulty(session.mode), s.rng)
			if err != nil {
				return err
			}
			if err := s.selectAttack(index); err != nil {
				return err
			}
		case "defend":
			index, err := ai.ChoosePoolIndex(session.state, session.actor, false, &session.attackCard, webDifficulty(session.mode), s.rng)
			if err != nil {
				return err
			}
			if err := s.resolveBattle(index); err != nil {
				return err
			}
		case "seat":
			seat, err := ai.ChooseSeat(session.state, session.actor, webDifficulty(session.mode), s.rng)
			if err != nil {
				return err
			}
			s.assignSeat(session.actor, seat)
			s.beginRoundThree()
		default:
			return nil
		}
	}
	return nil
}

func (s *webServer) beginBattles() {
	session := s.session
	session.exchange = 0
	s.startBattle()
	s.handoffForAttacker()
}

func (s *webServer) startBattle() {
	session := s.session
	if session.exchange >= 3 {
		return
	}
	seat := game.SeatX
	if session.exchange == 1 {
		seat = game.SeatY
	}
	attacker, err := session.state.PlayerAtSeat(seat)
	if err != nil {
		session.message = err.Error()
		return
	}
	session.attacker = attacker
	session.actor = attacker
	attackerPlayer, _ := session.state.Player(attacker)
	defenderPlayer, _ := session.state.Player(1 - attacker)
	if len(attackerPlayer.Pool) == 0 || len(defenderPlayer.Pool) == 0 {
		session.message = "Battle skipped because a player has no remaining pile card."
		session.hasLastBattle = false
		session.phase = "result"
		return
	}
	if session.exchange == 2 {
		session.attackIndex = 0
		session.attackCard = attackerPlayer.Pool[0]
		session.phase = "defend"
		session.actor = 1 - attacker
		session.message = "The last card in each pile is played automatically."
		if len(defenderPlayer.Pool) == 1 {
			if err := s.resolveBattle(0); err != nil {
				session.message = err.Error()
			}
		}
		return
	}
	session.phase = "attack"
	session.message = fmt.Sprintf("Player %d attacks. Choose a card from their pile.", attacker+1)
}

func (s *webServer) handoffForAttacker() {
	session := s.session
	if session.mode == "hvh" && session.phase == "attack" {
		session.handoffTarget = session.attacker
		session.handoffPhase = "attack"
		session.phase = "handoff"
		session.message = fmt.Sprintf("Pass the screen to Player %d to attack.", session.attacker+1)
	}
}

func (s *webServer) selectAttack(index int) error {
	session := s.session
	attacker, _ := session.state.Player(session.attacker)
	if index < 0 || index >= len(attacker.Pool) {
		return game.ErrInvalidPoolCard
	}
	session.attackIndex = index
	session.attackCard = attacker.Pool[index]
	session.actor = 1 - session.attacker
	session.phase = "defend"
	session.message = fmt.Sprintf("Player %d revealed %s. The defender chooses a response.", session.attacker+1, session.attackCard.Code())
	if session.mode == "hvh" {
		session.handoffTarget = session.actor
		session.handoffPhase = "defend"
		session.phase = "handoff"
		session.message = fmt.Sprintf("Pass the screen to Player %d to defend. Attack revealed: %s.", session.actor+1, session.attackCard.Code())
		return nil
	}
	if isAI(session.mode, session.actor) {
		defendIndex, err := ai.ChoosePoolIndex(session.state, session.actor, false, &session.attackCard, webDifficulty(session.mode), s.rng)
		if err != nil {
			return err
		}
		return s.resolveBattle(defendIndex)
	}
	return nil
}

func (s *webServer) resolveBattle(defendIndex int) error {
	session := s.session
	defender := 1 - session.attacker
	defenderPlayer, _ := session.state.Player(defender)
	if defendIndex < 0 || defendIndex >= len(defenderPlayer.Pool) {
		return game.ErrInvalidPoolCard
	}
	defenseCard := defenderPlayer.Pool[defendIndex]
	attackerIndex := session.attacker
	attackCard := session.attackCard
	outcome, err := game.PlayExchange(&session.state, attackerIndex, session.attackIndex, defender, defendIndex)
	if err != nil {
		return err
	}
	session.lastAttack, session.lastDefense, session.lastOutcome = attackCard, defenseCard, outcome
	session.hasLastBattle = true
	slotIndex := (session.state.Round-1)*3 + session.exchange
	cell := jailCell{}
	switch outcome.Winner {
	case 0:
		cell.Card = cardPointer(defenseCard)
		cell.CapturedBy = s.colorName(attackerIndex)
	case 1:
		cell.Card = cardPointer(attackCard)
		cell.CapturedBy = s.colorName(defender)
	default:
		cell.Clash = true
	}
	if slotIndex >= 0 && slotIndex < len(session.slots) {
		session.slots[slotIndex] = cell
	}
	session.phase = "result"
	if outcome.Winner == -1 {
		session.message = fmt.Sprintf("Clash: %s (%d) ties %s (%d). Both cards are discarded.", attackCard.Code(), outcome.AttackerValue, defenseCard.Code(), outcome.DefenderValue)
	} else {
		winner := attackerIndex
		if outcome.Winner == 1 {
			winner = defender
		}
		session.message = fmt.Sprintf("%s wins this battle and captures %s.", s.colorName(winner), outcome.Captured.Code())
	}
	return nil
}

func (s *webServer) continueAfterBattle() error {
	session := s.session
	if session.exchange < 2 {
		session.exchange++
		session.hasLastBattle = false
		s.startBattle()
		s.handoffForAttacker()
		return nil
	}
	game.DiscardPools(&session.state)
	if session.state.Round == 3 {
		s.finishGame()
		return nil
	}
	swap := session.state.Round == 1
	if err := game.EndRound(&session.state, swap, true); err != nil {
		return err
	}
	session.state.Round++
	if session.state.Round == 3 {
		s.chooseRoundThreeSeat()
		return nil
	}
	session.actor = 0
	session.phase = "commit"
	session.message = "New round: choose three cards from your hand to commit."
	if session.mode == "hvh" {
		session.handoffTarget = 0
		session.handoffPhase = "commit"
		session.phase = "handoff"
		session.message = "Pass the screen to Player 1 for the next round."
	}
	return nil
}

func (s *webServer) chooseRoundThreeSeat() {
	session := s.session
	score1 := game.Score(session.state.P1.Jail, session.state.Bounty.Suit).Total
	score2 := game.Score(session.state.P2.Jail, session.state.Bounty.Suit).Total
	chooser := 0
	if score2 < score1 || (score1 == score2 && s.rng.Intn(2) == 1) {
		chooser = 1
	}
	session.actor = chooser
	session.phase = "seat"
	session.message = fmt.Sprintf("Player %d chooses whether to take seat X or Y for Round 3.", chooser+1)
}

func (s *webServer) assignSeat(playerIndex int, seat game.Seat) {
	player, _ := s.session.state.Player(playerIndex)
	opponent, _ := s.session.state.Player(1 - playerIndex)
	player.Seat, opponent.Seat = seat, otherSeat(seat)
}

func (s *webServer) beginRoundThree() {
	s.session.actor = 0
	s.session.phase = "commit"
	s.session.message = "Round 3: choose three cards from your hand to commit."
}

func (s *webServer) finishGame() {
	session := s.session
	p1 := game.Score(session.state.P1.Jail, session.state.Bounty.Suit)
	p2 := game.Score(session.state.P2.Jail, session.state.Bounty.Suit)
	session.winner, session.tiebreak = finalWinner(p1, p2, s.rng)
	session.phase = "gameover"
	session.message = "The three rounds are complete."
}

func (s *webServer) view() webView {
	if s.session == nil {
		return webView{Started: false, Mode: s.defaultMode, Winner: -1}
	}
	session := s.session
	result := webView{
		Started:     true,
		Mode:        session.mode,
		Phase:       session.phase,
		Round:       session.state.Round,
		Exchange:    session.exchange + 1,
		Actor:       session.actor,
		Attacker:    session.attacker,
		Bounty:      session.state.Bounty,
		JailSlots:   make([]jailCell, len(session.slots)),
		Message:     session.message,
		CanContinue: session.phase == "result",
		Winner:      session.winner,
		Tiebreak:    session.tiebreak,
	}
	copy(result.JailSlots, session.slots[:])
	if session.phase == "defend" || session.phase == "result" && session.hasLastBattle {
		result.BattleAttack = cardPointer(session.attackCard)
	}
	if session.phase == "handoff" && session.handoffPhase == "defend" {
		result.BattleAttack = cardPointer(session.attackCard)
	}
	if session.phase == "result" && session.hasLastBattle {
		result.BattleDefend = cardPointer(session.lastDefense)
	}
	scores := [2]game.ScoreBreakdown{
		game.Score(session.state.P1.Jail, session.state.Bounty.Suit),
		game.Score(session.state.P2.Jail, session.state.Bounty.Suit),
	}
	result.Players = make([]webPlayer, 2)
	for i, player := range session.state.Players() {
		color := s.colorName(i)
		private := session.mode == "hvh" && i == session.actor && session.phase != "handoff" ||
			session.mode != "hvh" && i == 0
		view := webPlayer{
			Color: color, You: session.mode != "hvh" && i == 0, Seat: player.Seat.String(), Score: scores[i].Total,
			Hand: []webCard{}, Pool: []webCard{}, HiddenHand: len(player.Hand), HiddenPool: len(player.Pool),
		}
		if (session.phase == "defend" || session.phase == "handoff" && session.handoffPhase == "defend") && i == session.attacker {
			view.HiddenPool--
		}
		if private {
			view.Hand = make([]webCard, len(player.Hand))
			for cardIndex, card := range player.Hand {
				view.Hand[cardIndex] = webCard{Card: card, Index: cardIndex, Selectable: session.phase == "commit"}
			}
			view.HiddenHand = 0
			view.Pool = make([]webCard, 0, len(player.Pool))
			for cardIndex, card := range player.Pool {
				if session.phase == "defend" && i == session.attacker && cardIndex == session.attackIndex {
					continue
				}
				view.Pool = append(view.Pool, webCard{Card: card, Index: cardIndex, Selectable: (session.phase == "attack" && i == session.attacker) || (session.phase == "defend" && i != session.attacker)})
			}
			view.HiddenPool = len(player.Pool) - len(view.Pool)
			view.CanSelectHand = session.phase == "commit"
			view.CanSelectPool = (session.phase == "attack" && i == session.attacker) || (session.phase == "defend" && i != session.attacker)
		}
		result.Players[i] = view
	}
	if session.phase == "gameover" && session.mode != "hvh" {
		if session.winner == 0 {
			result.Message = "You Won!"
		} else {
			result.Message = "You Lost."
		}
		if session.phase == "gameover" && session.mode == "hvh" {
			result.Message = fmt.Sprintf("%s wins.", s.colorName(session.winner))
		}
	}
	if session.phase == "seat" && session.actor == 0 {
		result.Message = "You choose a seat for Round 3."
	}
	if session.phase == "handoff" {
		result.Message = fmt.Sprintf("Pass the screen to Player %d.", session.handoffTarget+1)
	}
	result.ScoreLine = fmt.Sprintf("%s = %d  |  %s = %d",
		s.scoreLabel(session.whitePlayer), scores[session.whitePlayer].Total,
		s.scoreLabel(1-session.whitePlayer), scores[1-session.whitePlayer].Total)
	return result
}

func (s *webServer) colorName(playerIndex int) string {
	if playerIndex == s.session.whitePlayer {
		return "White"
	}
	return "Black"
}

func (s *webServer) scoreLabel(playerIndex int) string {
	if s.session.mode != "hvh" && playerIndex == 0 {
		return "You"
	}
	return s.colorName(playerIndex)
}

func validWebMode(mode string) bool {
	return mode == "hvh" || mode == "easy" || mode == "normal" || mode == "hard"
}

func isAI(mode string, playerIndex int) bool {
	return mode != "hvh" && playerIndex == 1
}

func webDifficulty(mode string) ai.Difficulty {
	switch mode {
	case "easy":
		return ai.Easy
	case "normal":
		return ai.Normal
	default:
		return ai.Hard
	}
}

func decodeJSON(r *http.Request, value any) error {
	defer r.Body.Close()
	decoder := json.NewDecoder(io.LimitReader(r.Body, 1<<20))
	if err := decoder.Decode(value); err != nil {
		return fmt.Errorf("invalid request: %w", err)
	}
	return nil
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(value); err != nil {
		fmt.Fprintf(os.Stderr, "encode web response: %v\n", err)
	}
}

func writeError(w http.ResponseWriter, status int, err error) {
	writeJSON(w, status, map[string]string{"error": err.Error()})
}

func cardPointer(card game.Card) *game.Card { return &card }
