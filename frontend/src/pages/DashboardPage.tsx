import { useCallback, useEffect, useMemo, useState } from 'react';
import { Link, Navigate } from 'react-router-dom';
import {
  fetchMyAlerts,
  fetchMyWatches,
  removeWatch,
  runScan,
  type Alert,
  type ScanStats,
  type Watch,
} from '../api';
import { useAuth } from '../auth';
import { AddWatchForm } from '../components/AddWatchForm';
import { PriceChart } from '../components/PriceChart';
import { WatchBoard } from '../components/WatchBoard';

export function DashboardPage() {
  const { user, loading: authLoading, logout } = useAuth();
  const [watches, setWatches] = useState<Watch[]>([]);
  const [alerts, setAlerts] = useState<Alert[]>([]);
  const [selectedId, setSelectedId] = useState<string | null>(null);
  const [status, setStatus] = useState<'loading' | 'ready' | 'error' | 'empty'>('loading');
  const [stats, setStats] = useState<ScanStats | null>(null);
  const [busy, setBusy] = useState(false);
  const [toast, setToast] = useState<string | null>(null);

  const refresh = useCallback(async () => {
    try {
      const [w, a] = await Promise.all([fetchMyWatches(), fetchMyAlerts()]);
      setWatches(w);
      setAlerts(a);
      setSelectedId((prev) => prev ?? w[0]?.id ?? null);
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

  // After creating a watch, poll a few times while the background scan fills offers.
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
    await removeWatch(id);
    if (selectedId === id) setSelectedId(null);
    await refresh();
  }

  return (
    <div className="app-shell">
      <div className="blob blob-a" aria-hidden />
      <div className="blob blob-b" aria-hidden />
      <div className="blob blob-c" aria-hidden />

      <div className="app-content">
        <header className="topbar">
          <Link to="/" className="brand">
            <span className="brand-mark">Fw</span>
            <div>
              <div className="brand-name">FareWatch</div>
              <div className="brand-tag">{user.email}</div>
            </div>
          </Link>
          <div style={{ display: 'flex', gap: '0.5rem', alignItems: 'center', paddingRight: '0.35rem' }}>
            <button className="btn btn-outline" type="button" onClick={onScan} disabled={busy}>
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
            This dashboard is only the routes you asked us to email about. Each row is a full flight
            offer — flight number, schedule, aircraft, stops, price — from the airline poller.
          </p>
          {stats && (
            <p style={{ marginTop: '0.75rem', color: 'var(--fw-primary)', fontWeight: 700 }}>
              Last scan: {stats.faresFound} offers across {stats.airlinesQueried} airline APIs · Redis
              hit rate {stats.cacheHitRate.toFixed(0)}% · {stats.durationMs}ms
            </p>
          )}
        </section>

        <div className="layout">
          <WatchBoard
            watches={watches}
            selectedId={selectedId}
            onSelect={setSelectedId}
            onRemove={onRemove}
            status={status}
            onRetry={refresh}
          />
          <div className="stack">
            <AddWatchForm
              onCreated={() => {
                setStatus('loading');
                refresh();
              }}
            />
            <PriceChart watch={selected} />

            <div className="panel" style={{ borderRadius: '2rem 2.5rem 2rem 3rem' }}>
              <div className="panel-head">
                <h2>Recent alerts</h2>
                <span>Email</span>
              </div>
              <div className="panel-body">
                {!alerts.length && (
                  <p style={{ color: 'var(--fw-muted-foreground)' }}>
                    No alerts yet. When a fare crosses your threshold, mail goes to {user.email}.
                  </p>
                )}
                {alerts.map((a) => (
                  <div
                    key={a.id}
                    style={{
                      display: 'flex',
                      justifyContent: 'space-between',
                      gap: '1rem',
                      padding: '0.75rem 0',
                      borderBottom: '1px solid color-mix(in srgb, var(--fw-border) 50%, transparent)',
                    }}
                  >
                    <div>
                      <strong style={{ color: 'var(--fw-primary)' }}>
                        ${Math.round(a.oldPrice)} → ${Math.round(a.newPrice)}
                      </strong>{' '}
                      on {a.airline}
                      <div style={{ color: 'var(--fw-muted-foreground)', fontSize: '0.85rem' }}>
                        Delivered in {a.deliveredInMs}ms
                      </div>
                    </div>
                    <div style={{ color: 'var(--fw-muted-foreground)', fontSize: '0.85rem' }}>
                      {new Date(a.sentAt).toLocaleString()}
                    </div>
                  </div>
                ))}
              </div>
            </div>
          </div>
        </div>
      </div>

      {toast && (
        <div
          style={{
            position: 'fixed',
            right: '1rem',
            bottom: '1rem',
            background: 'var(--fw-card)',
            border: '1px solid var(--fw-border)',
            borderRadius: '1.5rem',
            padding: '0.9rem 1.1rem',
            boxShadow: 'var(--fw-shadow-float)',
            zIndex: 30,
            maxWidth: '22rem',
          }}
        >
          {toast}
        </div>
      )}
    </div>
  );
}
