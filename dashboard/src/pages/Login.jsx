import { useState } from 'react';
import { useAuth } from '../auth/AuthContext.jsx';
import { Button, Input } from '../components/ui.jsx';

export default function Login() {
  const { login } = useAuth();
  const [username, setUsername] = useState('');
  const [password, setPassword] = useState('');
  const [error, setError] = useState('');
  const [busy, setBusy] = useState(false);

  const submit = async (e) => {
    e.preventDefault();
    setError('');
    setBusy(true);
    try {
      await login(username, password);
    } catch (err) {
      setError(err.message || 'Login failed');
    } finally {
      setBusy(false);
    }
  };

  return (
    <div className="flex min-h-screen items-center justify-center p-4">
      <form onSubmit={submit} className="nb-card w-full max-w-md bg-white p-6 shadow-brutal-xl">
        <div className="mb-6 text-center">
          <div className="inline-block bg-brand px-3 py-1 text-2xl font-bold border-2 border-ink shadow-brutal">
            KALIA STORE
          </div>
          <p className="mt-3 font-bold uppercase tracking-wide">Admin Dashboard</p>
        </div>

        {error && (
          <div className="nb-card mb-4 bg-blaze p-3 font-bold text-white shadow-brutal-sm">{error}</div>
        )}

        <div className="space-y-4">
          <Input
            label="Username"
            value={username}
            onChange={(e) => setUsername(e.target.value)}
            autoFocus
            required
          />
          <Input
            label="Password"
            type="password"
            value={password}
            onChange={(e) => setPassword(e.target.value)}
            required
          />
          <Button type="submit" variant="primary" className="w-full" disabled={busy}>
            {busy ? 'Signing in…' : 'Sign In'}
          </Button>
        </div>
      </form>
    </div>
  );
}
