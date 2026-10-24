import React, { useEffect, useState } from 'react';
import { request } from './api';

export function DevelopmentAccounts({ onLogin, busy, setBusy }) {
  const [users, setUsers] = useState([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const [query, setQuery] = useState('');
  const [selected, setSelected] = useState('');
  const [disabled, setDisabled] = useState(false);

  async function load() {
    setLoading(true);
    setError('');
    try {
      const all = [];
      let after = '';
      do {
        const page = await request(`auth/users${after ? `?after=${encodeURIComponent(after)}` : ''}`);
        all.push(...page.users);
        after = page.next;
      } while (after);
      setUsers(all.sort((a, b) => a.email.localeCompare(b.email)));
    } catch (error) {
      if (error.status === 404) setDisabled(true);
      else setError(error.message);
    }
    finally { setLoading(false); }
  }

  useEffect(() => { load(); }, []);

  async function login(user) {
    setBusy(true);
    setSelected(user.id);
    setError('');
    try { onLogin((await request('auth/assume', { method: 'POST', body: { userId: user.id } })).user); }
    catch (error) { setError(error.message); }
    finally { setBusy(false); setSelected(''); }
  }

  const filtered = users.filter(user => `${user.email} ${user.role}`.toLowerCase().includes(query.toLowerCase()));
  if (disabled) return null;
  return <aside className="development-accounts" aria-labelledby="development-accounts-title">
    <span className="development-label">Development workspace</span>
    <h2 id="development-accounts-title">Sign in as any user</h2>
    <p>Choose an account to open its commander, games and permissions immediately.</p>
    <label>Find an account<input type="search" value={query} onChange={event => setQuery(event.target.value)} placeholder="Email or role" /></label>
    {error && <p role="alert">{error}</p>}
    {loading ? <p role="status">Loading accounts…</p> : <>
      <p role="status">{filtered.length} of {users.length} accounts</p>
      <ul className="account-picker">{filtered.map(user => <li key={user.id}>
        <button type="button" disabled={busy} onClick={() => login(user)}>
          <span><strong>{user.email}</strong><small>{user.role === 'admin' ? 'Administrator' : 'Player'}</small></span>
          <span>{selected === user.id ? 'Signing in…' : 'Sign in →'}</span>
        </button>
      </li>)}</ul>
      {!filtered.length && <p>{users.length ? 'No accounts match. Clear the search to see everyone.' : 'No accounts yet. Create the first account using the form.'}</p>}
    </>}
    <button type="button" className="secondary" disabled={loading || busy} onClick={load}>Refresh accounts</button>
  </aside>;
}
