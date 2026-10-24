export async function request(path, { method = 'GET', body } = {}) {
  const response = await fetch(`${import.meta.env.BASE_URL}api/${path}`, {
    method,
    credentials: 'same-origin',
    headers: body ? { 'Content-Type': 'application/json' } : {},
    body: body ? JSON.stringify(body) : undefined,
  });
  const text = await response.text();
  let data;
  try { data = JSON.parse(text); } catch { data = {}; }
  if (!response.ok) {
    const error = new Error(data.error || `Request failed (${response.status})`);
    error.status = response.status;
    if (response.status === 401 && !path.startsWith('auth/')) {
      window.dispatchEvent(new Event('pow-session-expired'));
    }
    throw error;
  }
  return data;
}
