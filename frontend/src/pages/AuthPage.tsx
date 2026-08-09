import { useEffect, useState, type FormEvent } from 'react';
import { Link, Navigate, useNavigate, useSearchParams } from 'react-router-dom';
import { createWatch, login, loginWithFirebase, register } from '../api';
import { useAuth } from '../auth';
import { firebaseEnabled, signInWithGoogle } from '../firebase';
import { clearPendingWatch, loadPendingWatch } from '../pendingWatch';
import styles from './AuthPage.module.css';

export function AuthPage({ mode }: { mode: 'login' | 'register' }) {
  const { user, setUser, loading } = useAuth();
  const navigate = useNavigate();
  const [params] = useSearchParams();
  const [name, setName] = useState('');
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [completing, setCompleting] = useState(false);
  const showGoogle = firebaseEnabled();
  const pending = loadPendingWatch();
  const fromWatch = params.get('next') === 'watch' || !!pending;

  async function finishAuth() {
    const selected = loadPendingWatch();
    if (selected) {
      await createWatch({
        origin: selected.origin,
        destination: selected.destination,
        departDate: selected.departDate,
        returnDate: selected.returnDate,
        cabin: selected.cabin,
        airlineCode: selected.airlineCode,
        flightNumber: selected.flightNumber,
        targetPrice: selected.targetPrice,
        dropPercent: 5,
      });
      clearPendingWatch();
    }
    navigate('/dashboard');
  }

  useEffect(() => {
    if (loading || !user || !fromWatch || completing || busy) return;
    setCompleting(true);
    finishAuth().catch((err) => {
      setCompleting(false);
      setError(err instanceof Error ? err.message : 'Could not start watch');
    });
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [loading, user, fromWatch]);

  if (!loading && user && !fromWatch) return <Navigate to="/dashboard" replace />;
  if (!loading && user && fromWatch && !error) {
    return (
      <div className="app-shell">
        <div className="app-content">
          <p style={{ padding: '3rem 1rem', textAlign: 'center' }}>Saving your selected flight…</p>
        </div>
      </div>
    );
  }

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
      await finishAuth();
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
      await finishAuth();
    } catch (err) {
      const code =
        err && typeof err === 'object' && 'code' in err
          ? String((err as { code?: string }).code ?? '')
          : '';
      if (code === 'auth/popup-closed-by-user' || code === 'auth/cancelled-popup-request') {
        setError('Google popup was closed before sign-in finished. Try again.');
      } else if (code === 'auth/unauthorized-domain') {
        setError('This domain is not allowed in Firebase Auth settings (add localhost).');
      } else if (code === 'auth/operation-not-allowed') {
        setError('Google sign-in is not enabled yet in Firebase Console → Authentication.');
      } else {
        setError(err instanceof Error ? err.message : 'Google sign-in failed');
      }
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
              {pending
                ? `Almost there. We’ll start watching ${pending.airline} ${pending.flightNumber} (${pending.origin} → ${pending.destination}) after you ${mode === 'register' ? 'register' : 'sign in'}.`
                : mode === 'register'
                  ? 'Save trips you care about and get an email when fares drop.'
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
                  {busy
                    ? 'Working…'
                    : mode === 'register'
                      ? pending
                        ? 'Create account & watch'
                        : 'Create account'
                      : pending
                        ? 'Sign in & watch'
                        : 'Sign in'}
                </button>
              </div>
            </form>

            {error && <p className={styles.error}>{error}</p>}

            <p className={styles.switch}>
              {mode === 'register' ? (
                <>
                  Already have an account?{' '}
                  <Link to={pending ? '/login?next=watch' : '/login'}>Sign in</Link>
                </>
              ) : (
                <>
                  New here?{' '}
                  <Link to={pending ? '/register?next=watch' : '/register'}>Create an account</Link>
                </>
              )}
            </p>
          </div>
        </div>
      </div>
    </div>
  );
}
