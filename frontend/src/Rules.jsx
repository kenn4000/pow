import React from 'react';

export function Rules() {
  return <details className="rules-guide">
    <summary>How to play · rules and rewards</summary>
    <ol>
      <li>Play three rounds of three exchanges. Seat X receives six cards and Seat Y five. Choose three cards for your round pile.</li>
      <li><strong>Scissors beats Paper; Paper beats Rock; Rock beats Scissors.</strong> A winning suit doubles its face value. Heroes can change these values.</li>
      <li>Seat X attacks first and third, Seat Y second. The defender sees the attack before responding. The final pair plays automatically.</li>
      <li>The higher effective value captures the opposing card. The winner’s card is discarded. Equal values clash: both cards are discarded.</li>
      <li>Each captured suit scores its weighted prisoner count × its highest scoring face. Shift changes faces; Bounty changes scoring faces in the bounty suit. Round weights change capture counts.</li>
      <li>Seats swap after Round 1. The lower-scoring player chooses a seat before Round 3. Hands refill between rounds.</li>
    </ol>
    <p>Marks buy cosmetics and boxes. Crowns buy heroes, upgrades and premium cosmetics. Win matches to work toward Camp Draws; achievements and campaigns offer additional rewards. Skill Points are an alternative way to unlock skills.</p>
    <p>Start with an Easy match or Basic Training in Campaigns. Local two-player mode shares this device and hides hands during handoffs.</p>
  </details>;
}
