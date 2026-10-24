import React, { useState } from 'react';
import { request } from './api';

export function PasswordSettings() {
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');
  const [message, setMessage] = useState('');

  async function submit(event) {
    event.preventDefault();
    const form = event.currentTarget;
    const values = new FormData(form);
    const newPassword = values.get('newPassword');
    setError('');
    setMessage('');
    if (newPassword !== values.get('confirmPassword')) {
      setError('The new passwords do not match.');
      return;
    }
    const length = new TextEncoder().encode(newPassword).length;
    if (length < 12 || length > 72) {
      setError('The new password must be between 12 and 72 bytes.');
      return;
    }
    setBusy(true);
    try {
      await request('account/password', { method: 'POST', body: { currentPassword: values.get('currentPassword'), newPassword } });
      form.reset();
      setMessage('Password changed. Other sessions have been signed out.');
    } catch (error) { setError(error.message); }
    finally { setBusy(false); }
  }

  return <main className="account-page password-page">
    <h1>Change password</h1>
    <p>Confirm your current password. Your current session stays signed in; other sessions are revoked.</p>
    <form onSubmit={submit}>
      <label>Current password<input name="currentPassword" type="password" autoComplete="current-password" required minLength={12} maxLength={72} /></label>
      <label>New password<input name="newPassword" type="password" autoComplete="new-password" required minLength={12} maxLength={72} /></label>
      <label>Confirm new password<input name="confirmPassword" type="password" autoComplete="new-password" required minLength={12} maxLength={72} /></label>
      <small>Use a different password, between 12 and 72 bytes.</small>
      {error && <p role="alert">{error}</p>}
      {message && <p role="status">{message}</p>}
      <button type="submit" disabled={busy}>{busy ? 'Changing password…' : 'Change password'}</button>
    </form>
  </main>;
}
