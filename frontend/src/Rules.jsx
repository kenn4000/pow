import React from 'react';

export function Rules() {
  return <details className="rules-guide">
    <summary>How to play · rules and rewards</summary>
    <h3>Shared deck and combat</h3>
    <ul>
      <li>The 27-card deck has Scissors, Paper, and Rock suits, numbered 1–9. Scissors beats Paper, Paper beats Rock, and Rock beats Scissors. A card that wins by suit doubles its face value before hero modifiers.</li>
      <li>One card sets the bounty suit. Hero Shift changes combat faces, Favor adjusts cycle-winning cards, and Bounty changes the scoring face of cards in the bounty suit.</li>
      <li>In standard Prisoners of War, both players receive private cards and alternate picks from a shared public pool, starting with Seat Y. Each round, secretly commit three cards. Exchanges resolve in X/Y/X attack order; the defender sees the attack before choosing a response. The higher effective value captures the opposing card; ties discard both cards.</li>
      <li>Each suit scores its weighted prisoner count multiplied by its highest scoring face. Round weights may change capture values. Ties are settled by distinct suits, then highest scoring face, then a coin flip.</li>
    </ul>

    <h3>Prisoners of War and Fog of War</h3>
    <p>Play against a CPU or another player online. Fog of War uses the same combat and scoring rules, but hidden opponent cards show only a face-value range until they are revealed. Standard play is also available as a best-of-three ranked series.</p>

    <h3>Solo campaigns</h3>
    <p>Campaigns are single-player missions against CPU opponents. They use the rules and layout of their variant, with scripted setups and mission-specific goals. Basic and Advanced Training teach standard combat and hero modifiers; the other campaigns teach these modes:</p>
    <ul>
      <li><strong>Abandon:</strong> Flip hidden board cards, then pair two unused cards from the board or your private pile. Standard combat determines captures; ties capture nothing. Captured cards score by suit, while unused private-pile cards can cost points.</li>
      <li><strong>Ladder:</strong> Play a legal card that meets or exceeds the current chain value. Wild cards change the chain suit. If no card is playable, pass; the opponent can open a fresh chain. Empty your hand to win.</li>
      <li><strong>Rummy:</strong> Build the strongest hand of melds. Sets are three cards of the same face in different suits; runs are three or more consecutive cards of one suit. Draft changes hand size, while hero modifiers affect meld and deadwood scoring.</li>
      <li><strong>Poker:</strong> Arrange cards into three balanced five-card hands. Place a card in a legal hand or discard it to draw a replacement. Meet the mission's scoring target and any hand constraints.</li>
      <li><strong>Puzzler:</strong> Solve a fixed card-pairing challenge. Pair cards to resolve standard combat and capture scoring, then meet the puzzle's target.</li>
      <li><strong>Tug War:</strong> Play cards against the top card of a shared pile, flip a deck card onto a pile, or spend a push to move a pile. Win clashes to pull piles to your side and score the cards in piles you own.</li>
    </ul>

    <h3>Standalone variants</h3>
    <ul>
      <li><strong>Abandon:</strong> Play solo against a randomized board and personal pile. Flip hidden board cards, then pair two unused cards from the board or your pile. Captured cards score by suit; unused personal cards can cost points.</li>
      <li><strong>Abandon Duel:</strong> Play a CPU or online opponent across two rounds. Pair two unused cards from the shared board or your own private pile; the second round reverses the starting player. Your total score across both rounds decides the winner.</li>
      <li><strong>Ladder:</strong> Play the chain-climbing game against a CPU or online opponent. The first player to empty their hand wins.</li>
      <li><strong>Rummy:</strong> Play a CPU or online opponent. Player 1 opens by drawing two cards, discarding one face-up and burying one. Each player gets six turns total, including the opening turn: take the face-up card (the deck reveals a replacement) or draw two, then discard one face-up and bury one. The best sets and same-suit runs score points; unmelded cards subtract points.</li>
      <li><strong>Poker Duel:</strong> Play a CPU or online opponent in a standalone match. Build three balanced five-card hands from a six-card hand, placing a card or discarding and redrawing. Turns are timed, and unused time banks toward a bonus.</li>
      <li><strong>Puzzle Battle:</strong> Play solo practice against the Easy CPU, or play another player online. Choose card pairs over two rounds—three pairs, then four—before the shared timer expires. Round-one scores stay hidden until both players submit.</li>
      <li><strong>Tug War:</strong> Play a CPU or online opponent. On each turn, play a hand card against a pile's top card, flip the deck onto a pile, or spend a limited push. When the deck is empty and neither player can win a hand play, consecutive passes end the game. Owned piles score their cards, with a bonus for bounty-suit cards.</li>
    </ul>

    <p>Marks buy cosmetics and boxes. Crowns buy heroes, upgrades, premium cosmetics, and Skill Points. Win matches to work toward Camp Draws; achievements and campaigns offer additional rewards. Skills can only be unlocked with Skill Points.</p>
    <p>Start with an Easy match or Basic Training in Campaigns. Human opponents are available by friend invitation or online matchmaking in supported modes.</p>
  </details>;
}
