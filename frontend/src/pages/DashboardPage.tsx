import { useCallback, useEffect, useMemo, useState } from 'react';
import { Link, Navigate } from 'react-router-dom';
import {
  fetchMyAlerts,
  fetchMyWatches,
  removeWatch,
  runScan,
  updateWatch,
  type Alert,
  type ScanStats,
  type Watch,
} from '../api';
import { useAuth } from '../auth';
import { PriceChart } from '../components/PriceChart';
import { WatchBoard } from '../components/WatchBoard';
import { formatPrice } from '../utils/price';
import styles from './DashboardPage.module.css';

export function DashboardPage() {
  const { user, loading: authLoading, logout } = useAuth();
  const [watches, setWatches] = useState<Watch[]>([]);
  const [alerts, setAlerts] = useState<Alert[]>([]);
  const [selectedId, setSelectedId] = useState<string | null>(null);
  const [status, setStatus] = useState<'loading' | 'ready' | 'error' | 'empty'>('loading');
  const [stats, setStats] = useState<ScanStats | null>(null);
  const [busy, setBusy] = useState(false);
  const [busyAction, setBusyAction] = useState<string | null>(null);
  const [toast, setToast] = useState<string | null>(null);

  const refresh = useCallback(async () => {
    try {
      const [w, a] = await Promise.all([fetchMyWatches(), fetchMyAlerts()]);
      setWatches(w);
      setAlerts(a);
      setSelectedId((prev) => {
        if (prev && w.some((item) => item.id === prev)) return prev;
        return w[0]?.id ?? null;
      });
      setStatus(w.length ? 'ready' : 'empty');
    } catch {
      setStatus('error');
    }
  }, []);

  useEffect(() => {
    if (!user) return;
    refresh();
  }, [user, refresh]);

  useEffect(() => {
    if (!toast) return;
    const t = window.setTimeout(() => setToast(null), 4000);
    return () => window.clearTimeout(t);
  }, [toast]);

  useEffect(() => {
    if (!user || status === 'error') return;
    const needsFare = watches.some((w) => !w.latestFare);
    if (!needsFare) return;
    const id = window.setInterval(() => {
      refresh();
    }, 2500);
    return () => window.clearInterval(id);
  }, [user, watches, status, refresh]);

  const selected = useMemo(
    () => watches.find((w) => w.id === selectedId) ?? null,
    [watches, selectedId],
  );

  if (authLoading) return null;
  if (!user) return <Navigate to="/login" replace />;

  async function onScan() {
    setBusy(true);
    try {
      const s = await runScan();
      setStats(s);
      await refresh();
      setToast(
        `Scan done · ${s.faresFound} offers · ${s.cacheHitRate.toFixed(0)}% Redis hits · ${s.airlinesQueried} airlines`,
      );
    } catch (err) {
      setToast(err instanceof Error ? err.message : 'Scan failed');
    } finally {
      setBusy(false);
    }
  }

  async function onRemove(id: string) {
    setBusyAction(`remove:${id}`);
    try {
      await removeWatch(id);
      setToast('Removed from watchlist — emails stopped for that route.');
      if (selectedId === id) setSelectedId(null);
      await refresh();
    } catch (err) {
      setToast(err instanceof Error ? err.message : 'Could not remove watch');
    } finally {
      setBusyAction(null);
    }
  }

  async function onToggleEmail(id: string, notifyOnDrop: boolean) {
    setBusyAction(`email:${id}`);
    try {
      await updateWatch({ id, notifyOnDrop });
      setToast(notifyOnDrop ? 'Email alerts turned back on.' : 'Email alerts muted for this watch.');
      await refresh();
    } catch (err) {
      setToast(err instanceof Error ? err.message : 'Could not update alerts');
    } finally {
      setBusyAction(null);
    }
  }

  return (
    <div className="app-shell">
      <div className="blob blob-a" aria-hidden />
      <div className="blob blob-b" aria-hidden />
      <div className="blob blob-c" aria-hidden />

      <div className="app-content">
        <header className={`topbar ${styles.topbar}`}>
          <Link to="/" className="brand">
            <span className="brand-mark">Fw</span>
            <div>
              <div className="brand-name">FareWatch</div>
              <div className="brand-tag">{user.email}</div>
            </div>
          </Link>
          <div className={`topbar-actions ${styles.topbarActions}`}>
            <Link className="btn btn-primary" to="/search">
              Search flights
            </Link>
            <button className="btn btn-outline desktop-action" type="button" onClick={onScan} disabled={busy}>
              {busy ? 'Scanning…' : 'Run scan'}
            </button>
            <button className="btn btn-ghost" type="button" onClick={logout}>
              Sign out
            </button>
          </div>
        </header>

        <section className="page-head">
          <h1>Hi, {user.name}</h1>
          <p>
            Your watchlist works like a fare ticker: pick a route, scrub the price trail, mute email or
            drop it from the board anytime.
          </p>
          {stats && (
            <p className={styles.scanLine}>
              Last scan: {stats.faresFound} offers across {stats.airlinesQueried} airline APIs · Redis
              hit rate {stats.cacheHitRate.toFixed(0)}% · {stats.durationMs}ms
            </p>
          )}
        </section>

        <div className={`layout ${styles.layout}`}>
          <WatchBoard
            watches={watches}
            selectedId={selectedId}
            onSelect={setSelectedId}
            onRemove={onRemove}
            status={status}
            onRetry={refresh}
          />
          <div className={`stack ${styles.stack}`}>
            <PriceChart
              watch={selected}
              onRemove={onRemove}
              onToggleEmail={onToggleEmail}
              busyAction={busyAction}
            />

            <div className={`panel ${styles.sidePanel}`}>
              <div className="panel-head">
                <h2>Find another fare</h2>
                <span>Search</span>
              </div>
              <div className="panel-body">
                <p className={styles.sideCopy}>
                  Compare itineraries, then add the one you want on the ticker.
                </p>
                <Link className="btn btn-primary" to="/search">
                  Search flights
                </Link>
              </div>
            </div>

            <div className={`panel ${styles.sidePanel}`}>
              <div className="panel-head">
                <h2>Recent alerts</h2>
                <span>Email</span>
              </div>
              <div className="panel-body">
                {!alerts.length && (
                  <p className={styles.sideCopy}>
                    No alerts yet. When a fare crosses your threshold, mail goes to {user.email}.
                  </p>
                )}
                {alerts.map((a) => (
                  <div key={a.id} className={styles.alertRow}>
                    <div>
                      <strong className={styles.alertPrice}>
                        {formatPrice(a.oldPrice)} → {formatPrice(a.newPrice)}
                      </strong>{' '}
                      on {a.airline}
                      <div className={styles.alertMeta}>Delivered in {a.deliveredInMs}ms</div>
                    </div>
                    <div className={styles.alertMeta}>{new Date(a.sentAt).toLocaleString()}</div>
                  </div>
                ))}
              </div>
            </div>
          </div>
        </div>
      </div>

      {toast && <div className={styles.toast}>{toast}</div>}
    </div>
  );
}
