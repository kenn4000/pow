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
import { DevelopmentAccounts } from './DevelopmentAccounts';
import { Rules } from './Rules';
import './ui.css';

function Game() {
  const host = useRef(null);

  useEffect(() => {
    const root = host.current;
    const events = new AbortController();
    const scopedDocument = {
      getElementById: id => root.querySelector(`#${id}`),
      querySelector: selector => root.querySelector(selector),
      querySelectorAll: selector => root.querySelectorAll(selector),
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
  const [visible, setVisible] = useState(false);
  useEffect(() => { document.title = `${signup ? 'Create account' : 'Sign in'} · Prisoners of War`; }, [signup]);

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

  return <main className="account-page auth-layout"><section className="auth-page">
    <div className="auth-brand">
      <span className="auth-crest" aria-hidden="true">⚔</span>
      <div><h1>Prisoners of War</h1><p className="auth-tagline">A tactical card game of Scissors, Paper and Rock</p></div>
    </div>
    <ul className="auth-suits" aria-label="Suit cycle">
      <li>✂ Scissors beats Paper</li><li>▤ Paper beats Rock</li><li>◆ Rock beats Scissors</li>
    </ul>
    <p>Choose your pile, capture prisoners and outscore your opponent over three rounds. Your commander, active game and match history stay with your account.</p>
    <Rules />
    <div className="auth-card">
      <h2>{signup ? 'Create an account' : 'Sign in'}</h2>
      <form onSubmit={submit}>
        <label>Email<input name="email" type="email" autoComplete="username" maxLength={254} required /></label>
        <label>Password<span className="password-field">
          <input name="password" type={visible ? 'text' : 'password'} minLength={signup ? 12 : undefined} maxLength={72} autoComplete={signup ? 'new-password' : 'current-password'} required />
          <button type="button" className="password-toggle" aria-pressed={visible} aria-label={visible ? 'Hide password' : 'Show password'} onClick={() => setVisible(!visible)}>{visible ? 'Hide' : 'Show'}</button>
        </span></label>
        {signup && <small>Use at least 12 characters. Maximum 72 UTF-8 bytes; accented characters and emoji can use more than one byte.</small>}
        {error && <p role="alert">{error}</p>}
        <button disabled={busy} type="submit">{busy ? 'Please wait…' : signup ? 'Create account' : 'Sign in'}</button>
      </form>
      <button className="secondary auth-switch" onClick={() => { setSignup(!signup); setError(''); }} disabled={busy}>
        {signup ? 'Already registered? Sign in' : 'New player? Create an account'}
      </button>
    </div>
  </section><DevelopmentAccounts onLogin={onLogin} busy={busy} setBusy={setBusy} /></main>;
}

function App() {
  const frontendBuild = { hash: import.meta.env.VITE_BUILD_HASH || 'unknown', date: import.meta.env.VITE_BUILD_DATE || 'unknown' };
  const [backendBuild, setBackendBuild] = useState({ hash: 'unknown', date: 'unknown' });
  const [user, setUser] = useState(undefined);
  const accountScreens = { 'saved-games': 'history', admin: 'admin', cards: 'cards', password: 'password' };
  const routeScreen = () => accountScreens[window.location.hash.split('/')[1]] || 'game';
  const [screen, setScreen] = useState(routeScreen);
  const [error, setError] = useState('');
  const [accountOpen, setAccountOpen] = useState(false);
  const [displayName, setDisplayName] = useState('');
  const accountBar = useRef(null);

  useEffect(() => {
    fetch(`${import.meta.env.BASE_URL}build-info`).then(response => response.ok ? response.json() : null).then(info => { if (info) setBackendBuild(info); }).catch(() => {});
  }, []);

  useEffect(() => {
    if (!accountBar.current) return;
    const observer = new ResizeObserver(([entry]) => {
      document.documentElement.style.setProperty('--account-height', `${entry.target.getBoundingClientRect().height}px`);
    });
    observer.observe(accountBar.current);
    return () => observer.disconnect();
  }, [user]);

  function navigate(path) {
    window.location.hash = `/${path}`;
    setAccountOpen(false);
  }

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
    const expire = () => { setUser(null); setDisplayName(''); };
    const route = () => { setScreen(routeScreen()); setAccountOpen(false); };
    const commander = event => setDisplayName(event.detail?.displayName || '');
    window.addEventListener('pow-session-expired', expire);
    window.addEventListener('hashchange', route);
    window.addEventListener('pow-commander', commander);
    return () => { window.removeEventListener('pow-session-expired', expire); window.removeEventListener('hashchange', route); window.removeEventListener('pow-commander', commander); };
  }, []);

  // Screens that do not mount the game (Saved games, Cards & rules, Password)
  // never receive a dashboard response, so load the commander name once per account.
  useEffect(() => {
    if (!user?.id) return undefined;
    let current = true;
    request('profile').then(data => { if (current) setDisplayName(data?.dashboard?.displayName || ''); }).catch(() => {});
    return () => { current = false; };
  }, [user?.id]);

  useEffect(() => {
    const titles = { history: 'Saved games', admin: 'User administration', cards: 'Card reference', password: 'Change password' };
    if (screen !== 'game') document.title = `${titles[screen]} · Prisoners of War`;
  }, [screen]);

  async function logout() {
    try {
      await request('auth/logout', { method: 'POST' });
      setUser(null);
      setDisplayName('');
      setScreen('game');
      navigate('home');
    } catch (error) { setError(error.message); }
  }

  if (user === undefined) return <main className="account-page"><p role="status">{error || 'Loading account…'}</p>{error && <button onClick={loadAccount}>Retry</button>}</main>;
  if (!user) return <AuthForm onLogin={setUser} />;
  return <>
    <header ref={accountBar} className={`account-bar${accountOpen ? ' account-open' : ''}`}>
      <span className="account-identity">{displayName
        ? <><strong className="account-name">{displayName}</strong><small className="account-email">{user.email}</small></>
        : user.email}{user.role === 'admin' && <small className="role-badge">Administrator</small>}</span>
      <button className="account-toggle" aria-expanded={accountOpen} aria-controls="account-navigation" onClick={() => setAccountOpen(!accountOpen)}>Account</button>
      <nav id="account-navigation" aria-label="Account navigation">
        <button aria-current={screen === 'game' ? 'page' : undefined} onClick={() => navigate('home')}>Game</button>
        {user.role !== 'local' && <button aria-current={screen === 'history' ? 'page' : undefined} onClick={() => navigate('saved-games')}>Saved games</button>}
        {user.role === 'admin' && <button aria-current={screen === 'admin' ? 'page' : undefined} onClick={() => navigate('admin')}>Admin</button>}
        <button aria-current={screen === 'cards' ? 'page' : undefined} onClick={() => navigate('cards')}>Cards & rules</button>
        {user.role !== 'local' && <span className="account-actions" role="group" aria-label="Account settings">
          <button aria-current={screen === 'password' ? 'page' : undefined} onClick={() => navigate('password')}>Password</button>
          <button onClick={logout}>Sign out</button>
        </span>}
      </nav>
      <section className="build-info" aria-label="Build information">
        <span className="build-info-title">Build information</span>
        <span className="build-version"><span>Frontend</span><code>{frontendBuild.hash}</code><time>{frontendBuild.date}</time></span>
        <span className="build-version"><span>Backend</span><code>{backendBuild.hash}</code><time>{backendBuild.date}</time></span>
      </section>
      {error && <p role="alert">{error}</p>}
    </header>
    {screen === 'admin' && user.role === 'admin'
      ? <AdminUsers currentUser={user} onRoleChange={(id, role) => { if (id === user.id) { setUser({ ...user, role }); if (role !== 'admin') navigate('home'); } }} />
      : screen === 'cards' ? <CardCatalog />
        : screen === 'password' ? <PasswordSettings />
          : screen === 'history' ? <SavedGames onResume={() => navigate('battle')} /> : <Game />}
  </>;
}

createRoot(document.getElementById('root')).render(<App />);
