import React, { useEffect, useRef, useState } from 'react';
import { createRoot } from 'react-dom/client';
import shell from 'virtual:game-shell';
import { mountGame } from 'virtual:game-controller';
import 'virtual:game-style.css';
import { request } from './api';
import { AdminUsers, SavedGames } from './pages';
import { PasswordSettings } from './PasswordSettings';
import { CardCatalog } from './CardCatalog';
import './account.css';

function Game() {
  const host = useRef(null);

  useEffect(() => {
    const root = host.current;
    const events = new AbortController();
    const scopedDocument = {
      getElementById: id => root.querySelector(`#${id}`),
      querySelector: selector => root.querySelector(selector),
      createElement: tag => document.createElement(tag),
      addEventListener: (type, listener) => root.addEventListener(type, listener, { signal: events.signal }),
    };
    const dispose = mountGame(scopedDocument);
    return () => {
      events.abort();
      dispose();
    };
  }, []);

  return <div ref={host} dangerouslySetInnerHTML={{ __html: shell }} />;
}

function AuthForm({ onLogin }) {
  const [signup, setSignup] = useState(false);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');

  async function submit(event) {
    event.preventDefault();
    setBusy(true);
    setError('');
    const values = new FormData(event.currentTarget);
    try {
      const data = await request(`auth/${signup ? 'signup' : 'login'}`, {
        method: 'POST', body: { email: values.get('email'), password: values.get('password') },
      });
      onLogin(data.user);
    } catch (error) { setError(error.message); }
    finally { setBusy(false); }
  }

  return <main className="account-page auth-page">
    <h1>Prisoners of War</h1>
    <p>Your commander, active game, and match history stay with your account.</p>
    <h2>{signup ? 'Create an account' : 'Sign in'}</h2>
    <form onSubmit={submit}>
      <label>Email<input name="email" type="email" autoComplete="username" maxLength={254} required /></label>
      <label>Password<input name="password" type="password" minLength={12} maxLength={72} autoComplete={signup ? 'new-password' : 'current-password'} required /></label>
      <small>Use a password between 12 and 72 bytes.</small>
      {error && <p role="alert">{error}</p>}
      <button disabled={busy} type="submit">{busy ? 'Please wait…' : signup ? 'Sign up' : 'Sign in'}</button>
    </form>
    <button className="secondary" onClick={() => { setSignup(!signup); setError(''); }} disabled={busy}>
      {signup ? 'Already registered? Sign in' : 'New player? Create an account'}
    </button>
  </main>;
}

function App() {
  const [user, setUser] = useState(undefined);
  const [screen, setScreen] = useState('game');
  const [error, setError] = useState('');

  async function loadAccount() {
    setError('');
    try { setUser((await request('auth/me')).user); }
    catch (error) {
      if (error.status === 401) setUser(null);
      else if (error.status === 404) setUser({ email: 'Local profile', role: 'local' });
      else setError(error.message);
    }
  }

  useEffect(() => {
    loadAccount();
    const expire = () => { setUser(null); setScreen('game'); };
    window.addEventListener('pow-session-expired', expire);
    return () => window.removeEventListener('pow-session-expired', expire);
  }, []);

  async function logout() {
    try {
      await request('auth/logout', { method: 'POST' });
      setUser(null);
      setScreen('game');
    } catch (error) { setError(error.message); }
  }

  if (user === undefined) return <main className="account-page"><p role="status">{error || 'Loading account…'}</p>{error && <button onClick={loadAccount}>Retry</button>}</main>;
  if (!user) return <AuthForm onLogin={setUser} />;
  return <>
    <header className="account-bar">
      <span>{user.email} <small>({user.role})</small></span>
      <nav aria-label="Account navigation">
        <button aria-current={screen === 'game' ? 'page' : undefined} onClick={() => setScreen('game')}>Game</button>
        {user.role !== 'local' && <button onClick={() => setScreen('history')}>Saved games</button>}
        {user.role === 'admin' && <button onClick={() => setScreen('admin')}>Admin</button>}
        {user.role === 'admin' && <button onClick={() => setScreen('cards')}>Cards</button>}
        {user.role !== 'local' && <button onClick={() => setScreen('password')}>Password</button>}
        {user.role !== 'local' && <button onClick={logout}>Sign out</button>}
      </nav>
      {error && <p role="alert">{error}</p>}
    </header>
    {screen === 'admin' && user.role === 'admin'
      ? <AdminUsers currentUser={user} onRoleChange={(id, role) => { if (id === user.id) { setUser({ ...user, role }); if (role !== 'admin') setScreen('game'); } }} />
      : screen === 'cards' && user.role === 'admin' ? <CardCatalog />
        : screen === 'password' ? <PasswordSettings />
          : screen === 'history' ? <SavedGames /> : <Game />}
  </>;
}

createRoot(document.getElementById('root')).render(<App />);
