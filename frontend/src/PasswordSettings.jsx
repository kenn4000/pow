import React, { useState } from 'react';
import { request } from './api';

export function PasswordSettings() {
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');
  const [message, setMessage] = useState('');
  const [visible, setVisible] = useState(false);
  const [mismatch, setMismatch] = useState(false);

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
      setMismatch(false);
      setMessage('Password changed. Other sessions have been signed out.');
    } catch (error) { setError(error.message); }
    finally { setBusy(false); }
  }

  return <main className="account-page password-page">
    <h1>Change password</h1>
    <p>Confirm your current password. Your current session stays signed in; other sessions are revoked.</p>
    <form onSubmit={submit}>
      <label>Current password<input name="currentPassword" type={visible ? 'text' : 'password'} autoComplete="current-password" required maxLength={72} /></label>
      <label>New password<input name="newPassword" type={visible ? 'text' : 'password'} autoComplete="new-password" required minLength={12} maxLength={72} onChange={event => { const form = event.target.form; setMismatch(Boolean(form.confirmPassword.value) && form.confirmPassword.value !== event.target.value); }} /></label>
      <label>Confirm new password<input name="confirmPassword" type={visible ? 'text' : 'password'} autoComplete="new-password" required minLength={12} maxLength={72} aria-invalid={mismatch} aria-describedby="password-confirmation-help" onChange={event => setMismatch(Boolean(event.target.value) && event.target.value !== event.target.form.newPassword.value)} /></label>
      <small id="password-confirmation-help" aria-live="polite">{mismatch ? 'The new passwords do not match.' : 'Repeat the new password to confirm.'}</small>
      <small>Use a different password with at least 12 characters, maximum 72 UTF-8 bytes. Emoji and accented characters may use multiple bytes.</small>
      <button type="button" className="visibility-toggle" aria-pressed={visible} onClick={() => setVisible(!visible)}>{visible ? 'Hide passwords' : 'Show passwords'}</button>
      {error && <p role="alert">{error}</p>}
      {message && <p role="status">{message}</p>}
      <button type="submit" disabled={busy}>{busy ? 'Changing password…' : 'Change password'}</button>
    </form>
  </main>;
}
