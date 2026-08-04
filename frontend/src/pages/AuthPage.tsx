import { useState, type FormEvent } from 'react';
import { Link, Navigate, useNavigate } from 'react-router-dom';
import { login, loginWithFirebase, register } from '../api';
import { useAuth } from '../auth';
import { firebaseEnabled, signInWithGoogle } from '../firebase';
import styles from './AuthPage.module.css';

export function AuthPage({ mode }: { mode: 'login' | 'register' }) {
  const { user, setUser, loading } = useAuth();
  const navigate = useNavigate();
  const [name, setName] = useState('');
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const showGoogle = firebaseEnabled();

  if (!loading && user) return <Navigate to="/dashboard" replace />;

  async function onSubmit(e: FormEvent) {
    e.preventDefault();
    setBusy(true);
    setError(null);
    try {
      const u =
        mode === 'register'
          ? await register({ email, password, name })
          : await login({ email, password });
      setUser(u);
      navigate('/dashboard');
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Auth failed');
    } finally {
      setBusy(false);
    }
  }

  async function onGoogle() {
    setBusy(true);
    setError(null);
    try {
      const idToken = await signInWithGoogle();
      const u = await loginWithFirebase(idToken);
      setUser(u);
      navigate('/dashboard');
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Google sign-in failed');
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className="app-shell">
      <div className="blob blob-a" aria-hidden />
      <div className="blob blob-b" aria-hidden />
      <div className="app-content">
        <header className="topbar">
          <Link to="/" className="brand">
            <span className="brand-mark">Fw</span>
            <span className="brand-name">FareWatch</span>
          </Link>
        </header>

        <div className={styles.wrap}>
          <div className={styles.card}>
            <h1>{mode === 'register' ? 'Create your account' : 'Welcome back'}</h1>
            <p>
              {mode === 'register'
                ? 'Save the trips you care about and get a calm email when fares drop.'
                : 'Sign in to see the routes you’re watching.'}
            </p>

            {showGoogle && (
              <>
                <button
                  className="btn btn-outline"
                  type="button"
                  onClick={onGoogle}
                  disabled={busy}
                  style={{ width: '100%', marginBottom: '1rem' }}
                >
                  Continue with Google
                </button>
                <p
                  style={{
                    textAlign: 'center',
                    color: 'var(--fw-muted-foreground)',
                    fontSize: '0.85rem',
                    marginBottom: '1rem',
                  }}
                >
                  or use email
                </p>
              </>
            )}

            <form className="form-grid" onSubmit={onSubmit} style={{ gridTemplateColumns: '1fr' }}>
              {mode === 'register' && (
                <div className="field full">
                  <label htmlFor="name">Name</label>
                  <input id="name" value={name} onChange={(e) => setName(e.target.value)} required />
                </div>
              )}
              <div className="field full">
                <label htmlFor="email">Email</label>
                <input
                  id="email"
                  type="email"
                  value={email}
                  onChange={(e) => setEmail(e.target.value)}
                  required
                />
              </div>
              <div className="field full">
                <label htmlFor="password">Password</label>
                <input
                  id="password"
                  type="password"
                  minLength={6}
                  value={password}
                  onChange={(e) => setPassword(e.target.value)}
                  required
                />
              </div>
              <div className="full">
                <button className="btn btn-primary" type="submit" disabled={busy} style={{ width: '100%' }}>
                  {busy ? 'Working…' : mode === 'register' ? 'Create account' : 'Sign in'}
                </button>
              </div>
            </form>

            {error && <p className={styles.error}>{error}</p>}

            <p className={styles.switch}>
              {mode === 'register' ? (
                <>
                  Already have an account? <Link to="/login">Sign in</Link>
                </>
              ) : (
                <>
                  New here? <Link to="/register">Create an account</Link>
                </>
              )}
            </p>
          </div>
        </div>
      </div>
    </div>
  );
}
