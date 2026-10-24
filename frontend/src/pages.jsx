import React, { useEffect, useState } from 'react';
import { request } from './api';

export function AdminUsers({ currentUser, onRoleChange }) {
  const [users, setUsers] = useState([]);
  const [next, setNext] = useState('');
  const [error, setError] = useState('');
  const [busy, setBusy] = useState(false);

  async function load(after = '') {
    setBusy(true);
    setError('');
    try {
      const data = await request(`admin/users${after ? `?after=${encodeURIComponent(after)}` : ''}`);
      setUsers(previous => after ? [...previous, ...data.users] : data.users);
      setNext(data.next);
    } catch (error) { setError(error.message); }
    finally { setBusy(false); }
  }
  useEffect(() => { load(); }, []);

  async function changeRole(user, role) {
    if (role === user.role) return;
    setBusy(true);
    setError('');
    try {
      await request(`admin/users/${user.id}/role`, { method: 'PATCH', body: { role } });
      setUsers(previous => previous.map(item => item.id === user.id ? { ...item, role } : item));
      onRoleChange(user.id, role);
    } catch (error) { setError(error.message); }
    finally { setBusy(false); }
  }

  return <main className="account-page">
    <h1>User administration</h1>
    <p>Users can play and view their own games. Administrators can also manage roles.</p>
    {error && <p role="alert">{error}</p>}
    <div className="table-scroll"><table>
      <thead><tr><th>Email</th><th>Registered</th><th>Role</th></tr></thead>
      <tbody>{users.map(user => <tr key={user.id}>
        <td>{user.email}{user.id === currentUser.id ? ' (you)' : ''}</td>
        <td>{new Date(user.createdAt).toLocaleDateString()}</td>
        <td><select aria-label={`Role for ${user.email}`} disabled={busy} value={user.role} onChange={event => changeRole(user, event.target.value)}>
          <option value="user">User</option><option value="admin">Admin</option>
        </select></td>
      </tr>)}</tbody>
    </table></div>
    {busy && <p role="status">Updating…</p>}
    <button disabled={busy} onClick={() => load()}>Refresh</button>
    {next && <button disabled={busy} onClick={() => load(next)}>Load more</button>}
  </main>;
}

export function SavedGames() {
  const [games, setGames] = useState([]);
  const [next, setNext] = useState('');
  const [events, setEvents] = useState([]);
  const [gameID, setGameID] = useState('');
  const [nextEvent, setNextEvent] = useState('');
  const [error, setError] = useState('');
  const [busy, setBusy] = useState(false);

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
  useEffect(() => { load(); }, []);

  async function inspect(id, after = '') {
    setBusy(true);
    setError('');
    try {
      const data = await request(`games/${id}${after ? `?after=${encodeURIComponent(after)}` : ''}`);
      setGameID(id);
      setEvents(previous => after ? [...previous, ...data.events] : data.events);
      setNextEvent(data.next);
    } catch (error) { setError(error.message); }
    finally { setBusy(false); }
  }

  return <main className="account-page">
    <h1>Saved games</h1>
    <p>Return to Game to resume your latest match. Starting another match archives the unfinished one.</p>
    {error && <p role="alert">{error}</p>}
    {!busy && !games.length && <p>No saved games yet.</p>}
    <ul className="saved-games">{games.map(game => <li key={game.id}>
      <strong>{game.mode} · {game.status}</strong> <time>{new Date(game.startedAt).toLocaleString()}</time>
      {game.record && <p>{game.record.won ? 'Won' : 'Lost'} · {game.record.player_score}–{game.record.opp_score} · +{game.record.marks_earned} Marks</p>}
      <button disabled={busy} onClick={() => inspect(game.id)}>View state history</button>
    </li>)}</ul>
    {next && <button disabled={busy} onClick={() => load(next)}>Older games</button>}
    <button disabled={busy} onClick={() => load()}>Refresh</button>
    {gameID && <section><h2>Game transitions</h2><ol>
      {events.map(event => <li key={event.sequence}>
        <details><summary>#{event.sequence} · Round {event.view.round} · {event.action.action || 'Start game'} · {event.view.phase}</summary>
          <p>{event.view.message}</p><pre>{JSON.stringify(event.view, null, 2)}</pre>
        </details>
      </li>)}
    </ol>{nextEvent && <button disabled={busy} onClick={() => inspect(gameID, nextEvent)}>More transitions</button>}</section>}
    {busy && <p role="status">Loading…</p>}
  </main>;
}
