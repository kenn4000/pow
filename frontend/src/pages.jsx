import React, { useEffect, useRef, useState } from 'react';
import { request } from './api';

const modeLabel = mode => ({ hvh: 'Local two-player · Legacy', online: 'Online PvP', easy: 'AI · Easy', normal: 'AI · Normal', hard: 'AI · Hard' })[mode] || mode;
const gameLabel = game => game.campaignName ? `Campaign · ${game.campaignName}` : modeLabel(game.mode);
const points = count => `${count} ${count === 1 ? 'point' : 'points'}`;
const statusLabel = status => ({ active: 'In progress', completed: 'Completed', abandoned: 'Archived unfinished' })[status] || status;
const cardLabel = card => card ? `${['Scissors', 'Paper', 'Rock'][card.suit]} ${card.face}` : '';

export function AdminUsers({ currentUser, onRoleChange }) {
  const [users, setUsers] = useState([]);
  const [next, setNext] = useState('');
  const [error, setError] = useState('');
  const [message, setMessage] = useState('');
  const [busy, setBusy] = useState(false);
  const [query, setQuery] = useState('');
  const [role, setRole] = useState('');
  const [drafts, setDrafts] = useState({});
  const loadVersion = useRef(0);

  async function load(after = '') {
    const version = ++loadVersion.current;
    setBusy(true);
    setError('');
    try {
      const params = new URLSearchParams({ after, q: query, role });
      const data = await request(`admin/users?${params}`);
      if (version !== loadVersion.current) return;
      setUsers(previous => after ? [...previous, ...data.users] : data.users);
      setNext(data.next);
    } catch (error) { if (version === loadVersion.current) setError(error.message); }
    finally { if (version === loadVersion.current) setBusy(false); }
  }
  useEffect(() => {
    const timer = setTimeout(() => load(), 200);
    return () => { clearTimeout(timer); loadVersion.current++; };
  }, [query, role]);

  async function changeRole(user) {
    const role = drafts[user.id] || user.role;
    if (role === user.role) return;
    if (user.id === currentUser.id && role !== 'admin' && !window.confirm('Change your account to Player? Administrator controls will no longer be available to you.')) return;
    setBusy(true);
    setError('');
    setMessage('');
    try {
      await request(`admin/users/${user.id}/role`, { method: 'PATCH', body: { role } });
      setUsers(previous => previous.map(item => item.id === user.id ? { ...item, role } : item));
      setMessage(`Saved ${user.email} as ${role === 'admin' ? 'Administrator' : 'Player'}.`);
      onRoleChange(user.id, role);
    } catch (error) { setError(`${user.email}: ${error.message}`); }
    finally { setBusy(false); }
  }

  return <main className="account-page">
    <h1>User administration</h1>
    <p>Players manage their own commander and matches. Administrators can also manage account roles. Select a role, then Save to apply it.</p>
    <div className="catalog-toolbar">
      <label>Search email<input type="search" value={query} onChange={event => setQuery(event.target.value)} /></label>
      <label>Filter role<select value={role} onChange={event => setRole(event.target.value)}><option value="">All roles</option><option value="user">Players</option><option value="admin">Administrators</option></select></label>
    </div>
    {error && <p role="alert">{error}</p>}
    {message && <p role="status">{message}</p>}
    <div className="table-scroll"><table>
      <thead><tr><th>Email</th><th>Registered</th><th>Role</th><th>Action</th></tr></thead>
      <tbody>{users.map(user => <tr key={user.id}>
        <td>{user.email}{user.id === currentUser.id ? ' (you)' : ''}</td>
        <td>{new Date(user.createdAt).toLocaleDateString()}</td>
        <td><select aria-label={`Role for ${user.email}`} disabled={busy} value={drafts[user.id] || user.role} onChange={event => setDrafts({ ...drafts, [user.id]: event.target.value })}>
          <option value="user">Player</option><option value="admin">Administrator</option>
        </select></td>
        <td><button disabled={busy || !drafts[user.id] || drafts[user.id] === user.role} onClick={() => changeRole(user)} aria-label={`Save role for ${user.email}`}>Save</button></td>
      </tr>)}</tbody>
    </table></div>
    {!busy && !users.length && <p>No accounts match your filters.</p>}
    {busy && <p role="status">Loading or saving accounts…</p>}
    <button disabled={busy} onClick={() => load()}>Refresh</button>
    {next && <button disabled={busy} onClick={() => load(next)}>Load more accounts</button>}
  </main>;
}

// Saved views carry engine wording ("Player 2", "White wins"). These helpers
// restate it with the same labels as the entry's score line: You/Opponent in
// AI games, Player 1/Player 2 in local games.
export function timelinePlayerName(view, index) {
  const player = view.players?.[index];
  if (view.mode === 'hvh' || !player) return `Player ${index + 1}`;
  return player.you ? 'You' : 'Opponent';
}

function verb(name, singular, plural) { return name === 'You' ? plural : singular; }

export function timelineMessage(view) {
  const players = view.players || [];
  const byColor = color => timelinePlayerName(view, players.findIndex(player => player.color === color));
  let message = String(view.message || '');
  if (view.mode !== 'hvh') {
    if (message === 'You Won!') return 'You won the match.';
    if (message === 'You Lost.') return 'You lost the match.';
  }
  message = message.replace(/^(White|Black) wins this battle and captures the opposing card\./, (_, color) => {
    const name = byColor(color);
    return `${name} ${verb(name, 'wins', 'win')} this battle and ${verb(name, 'captures', 'capture')} the opposing card.`;
  });
  message = message.replace(/Player ([12])/g, (_, number) => timelinePlayerName(view, Number(number) - 1));
  return message.replace(/\bYou (attacks|chooses|is|has)\b/g, (_, word) => `You ${{ attacks: 'attack', chooses: 'choose', is: 'are', has: 'have' }[word]}`);
}

export function timelineExchange(view) {
  if (!view.battleAttack || typeof view.attacker !== 'number') return '';
  const attacker = timelinePlayerName(view, view.attacker);
  const defender = timelinePlayerName(view, 1 - view.attacker);
  const outcome = view.battleOutcome;
  let text = `${attacker} attacked with ${cardLabel(view.battleAttack)}${outcome ? ` (value ${outcome.attackerValue})` : ''}`;
  if (view.battleDefend) text += ` · ${defender} defended with ${cardLabel(view.battleDefend)}${outcome ? ` (value ${outcome.defenderValue})` : ''}`;
  if (outcome && view.battleDefend) {
    if (outcome.winner === -1) text += ' · Clash: both cards discarded';
    else {
      const winner = outcome.winner === 0 ? attacker : defender;
      text += ` · ${winner} ${verb(winner, 'captures', 'capture')} the opposing card`;
    }
  } else if (!view.battleDefend) text += ` · ${defender} to respond`;
  return `${text}.`;
}

function Transition({ event }) {
  const view = event.view;
  const actions = { commit: 'Chose round pile', attack: 'Played attack', defend: 'Played defense', continue: 'Continued to next step', ready: 'Acknowledged player handoff', seat: 'Chose final-round seat' };
  const title = event.action.campaignId ? `Started mission${view.campaign?.name ? `: ${view.campaign.name}` : ''}` : actions[event.action.action] || 'Started match';
  const exchange = timelineExchange(view);
  const message = timelineMessage(view);
  // The exchange line already states the reveal, capture or clash; show the
  // engine message only when it adds something (hints, round changes, results).
  const repeatsExchange = exchange && /revealed a card|wins? this battle|Clash!/.test(message);
  return <li className="timeline-card">
    <strong>#{event.sequence} · Round {view.round} · {title}</strong>
    {!repeatsExchange && <p>{message}</p>}
    {exchange && <p className="timeline-exchange">{exchange}</p>}
    {view.players && <p className="timeline-score">{view.players.map((player, index) => `${timelinePlayerName(view, index)}: ${points(player.score)} (Seat ${player.seat}, ${player.color})`).join(' · ')}</p>}
    <details><summary>Technical state JSON</summary><pre>{JSON.stringify(view, null, 2)}</pre></details>
  </li>;
}

export function SavedGames({ onResume }) {
  const [games, setGames] = useState([]);
  const [next, setNext] = useState('');
  const [events, setEvents] = useState([]);
  const [selected, setSelected] = useState(null);
  const [nextEvent, setNextEvent] = useState('');
  const [error, setError] = useState('');
  const [busy, setBusy] = useState(false);
  const inspectVersion = useRef(0);
  const timeline = useRef(null);

  async function load(before = '') {
    setBusy(true);
    setError('');
    try {
      const data = await request(`games${before ? `?before=${encodeURIComponent(before)}` : ''}`);
      setGames(previous => before ? [...previous, ...data.games] : data.games);
      setNext(data.next);
    } catch (error) { setError(error.message); }
    finally { setBusy(false); }
  }
  useEffect(() => {
    load();
    const route = () => {
      const id = window.location.hash.split('/')[2];
      if (id) inspect({ id });
      else { inspectVersion.current++; setSelected(null); setEvents([]); }
    };
    route();
    window.addEventListener('hashchange', route);
    return () => { inspectVersion.current++; window.removeEventListener('hashchange', route); };
  }, []);

  async function inspect(game, after = '') {
    const version = ++inspectVersion.current;
    setBusy(true);
    setError('');
    if (!after) setEvents([]);
    try {
      const data = await request(`games/${game.id}${after ? `?after=${encodeURIComponent(after)}` : ''}`);
      if (version !== inspectVersion.current) return;
      setSelected(data.game || game);
      setEvents(previous => after ? [...previous, ...data.events] : data.events);
      setNextEvent(data.next);
      requestAnimationFrame(() => timeline.current?.focus());
    } catch (error) { if (version === inspectVersion.current) setError(error.message); }
    finally { if (version === inspectVersion.current) setBusy(false); }
  }

  return <main className="account-page">
    <h1>Saved games</h1>
    <p>Resume your active match or review a readable timeline. Starting a replacement archives the unfinished match; its history stays available.</p>
    {error && <p role="alert">{error}</p>}
    {!busy && !games.length && <p>No saved games yet. <a href="#/match/new">Play your first match</a>.</p>}
    <ul className="saved-games">{games.map(game => <li key={game.id} className={game.id === selected?.id ? 'selected' : ''}>
      <strong>{gameLabel(game)} · {statusLabel(game.status)}</strong> <time dateTime={game.startedAt}>{new Date(game.startedAt).toLocaleString()}</time>
      {game.record && <p>{game.record.won ? 'Won' : 'Lost'} · {game.record.player_score}–{game.record.opp_score} · <span className="currency-amount" aria-label={`+${game.record.marks_earned} Marks`}>+{game.record.marks_earned}<img className="currency-icon" src={`${import.meta.env.BASE_URL}skins/icons/marks.png`} alt="" aria-hidden="true" /></span></p>}
      {game.status === 'active' && <button disabled={busy} onClick={onResume}>Resume match</button>}{' '}
      <button aria-pressed={game.id === selected?.id} disabled={busy} onClick={() => { if (window.location.hash === `#/saved-games/${game.id}`) inspect(game); else window.location.hash = `/saved-games/${game.id}`; }}>Review match timeline</button>
    </li>)}</ul>
    {next && <button disabled={busy} onClick={() => load(next)}>Older games</button>}
    <button disabled={busy} onClick={() => load()}>Refresh</button>
    {selected && <section ref={timeline} tabIndex={-1} aria-label="Selected match timeline"><h2>{gameLabel(selected)} · {new Date(selected.startedAt).toLocaleString()}</h2>
      <p>{statusLabel(selected.status)} · Match timeline</p>
      <ol>{events.map(event => <Transition key={event.sequence} event={event} />)}</ol>
      {nextEvent && <button disabled={busy} onClick={() => inspect(selected, nextEvent)}>More transitions</button>}
    </section>}
    {busy && <p role="status">Loading…</p>}
  </main>;
}
