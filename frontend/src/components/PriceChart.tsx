import { useEffect, useMemo, useState } from 'react';
import { fetchFares, type Fare, type Watch } from '../api';
import { EmptyState, LoadingState } from './StatusStates';
import styles from './PriceChart.module.css';

type Props = {
  watch: Watch | null;
};

function money(n: number) {
  return `$${Math.round(n)}`;
}

export function PriceChart({ watch }: Props) {
  const [fares, setFares] = useState<Fare[]>([]);
  const [loading, setLoading] = useState(false);

  useEffect(() => {
    if (!watch?.routeId) {
      setFares([]);
      return;
    }
    setLoading(true);
    fetchFares(watch.routeId, 60)
      .then(setFares)
      .catch(() => setFares([]))
      .finally(() => setLoading(false));
  }, [watch?.routeId, watch?.latestFare?.observedAt]);

  const history = useMemo(() => {
    // Collapse to daily lows for chart (oldest → newest).
    const byDay = new Map<string, number>();
    [...fares].reverse().forEach((f) => {
      const day = f.observedAt.slice(0, 10);
      const prev = byDay.get(day);
      if (prev == null || f.price < prev) byDay.set(day, f.price);
    });
    return [...byDay.entries()].map(([date, price]) => ({ date, price }));
  }, [fares]);

  if (!watch) {
    return (
      <div className={`panel ${styles.panelOrganic}`}>
        <div className="panel-head">
          <h2>Fare trail</h2>
          <span>History</span>
        </div>
        <div className={styles.emptyPick}>
          <EmptyState
            title="Pick a watch"
            message="Select a route to see stored offer history and your alert line."
          />
        </div>
      </div>
    );
  }

  if (loading && !history.length) {
    return (
      <div className={`panel ${styles.panelOrganic}`}>
        <div className="panel-head">
          <h2>
            {watch.route?.origin} → {watch.route?.destination}
          </h2>
          <span>Loading</span>
        </div>
        <LoadingState />
      </div>
    );
  }

  const latest = watch.latestFare;
  const threshold = watch.targetPrice ?? latest?.price ?? 0;
  const width = 560;
  const height = 220;
  const pad = { top: 18, right: 16, bottom: 28, left: 40 };
  const prices = history.length ? history.map((p) => p.price) : latest ? [latest.price] : [threshold];
  const min = Math.min(...prices, threshold) * 0.96;
  const max = Math.max(...prices, threshold) * 1.04;
  const innerW = width - pad.left - pad.right;
  const innerH = height - pad.top - pad.bottom;
  const x = (i: number) => pad.left + (i / Math.max(history.length - 1, 1)) * innerW;
  const y = (price: number) => pad.top + ((max - price) / (max - min || 1)) * innerH;
  const line = history
    .map((p, i) => `${i === 0 ? 'M' : 'L'} ${x(i).toFixed(1)} ${y(p.price).toFixed(1)}`)
    .join(' ');
  const area = history.length
    ? `${line} L ${x(history.length - 1).toFixed(1)} ${pad.top + innerH} L ${x(0).toFixed(1)} ${pad.top + innerH} Z`
    : '';
  const thresholdY = y(threshold);
  const low = Math.min(...prices);
  const high = Math.max(...prices);

  return (
    <div className={`panel ${styles.panelOrganic}`}>
      <div className="panel-head">
        <h2>
          {watch.route?.origin} → {watch.route?.destination}
        </h2>
        <span>{history.length || 1} pts</span>
      </div>
      <div className={`panel-body ${styles.wrap}`}>
        {latest && (
          <div className={styles.summary} style={{ gridTemplateColumns: '1fr' }}>
            <div className={styles.stat}>
              <label>Best current offer</label>
              <strong>
                {latest.flightNumber} · {latest.airline} · {money(latest.price)}
              </strong>
              <div style={{ marginTop: '0.35rem', color: 'var(--fw-muted-foreground)', fontSize: '0.85rem' }}>
                {latest.origin} {new Date(latest.departAt).toLocaleString()} → {latest.destination}{' '}
                {new Date(latest.arriveAt).toLocaleString()} · {latest.aircraft} ·{' '}
                {latest.stops === 0 ? 'Nonstop' : `${latest.stops} stop`} · source {latest.source}
                {latest.cached ? ' (redis)' : ''}
              </div>
            </div>
          </div>
        )}

        <div className={styles.summary}>
          <div className={styles.stat}>
            <label>Low</label>
            <strong>{money(low)}</strong>
          </div>
          <div className={styles.stat}>
            <label>High</label>
            <strong>{money(high)}</strong>
          </div>
          <div className={styles.stat}>
            <label>Now</label>
            <strong>{money(latest?.price ?? threshold)}</strong>
          </div>
          <div className={styles.stat}>
            <label>Alert at</label>
            <strong style={{ color: 'var(--fw-secondary)' }}>{money(threshold)}</strong>
          </div>
        </div>

        <svg className={styles.chart} viewBox={`0 0 ${width} ${height}`} role="img" aria-label="Fare history chart">
          {[0, 0.25, 0.5, 0.75, 1].map((t) => {
            const gy = pad.top + innerH * t;
            const label = Math.round(max - (max - min) * t);
            return (
              <g key={t}>
                <line x1={pad.left} x2={width - pad.right} y1={gy} y2={gy} stroke="rgba(222, 216, 207, 0.7)" />
                <text x={4} y={gy + 4} fill="#78786C" fontSize="10" fontFamily="Nunito, sans-serif">
                  {label}
                </text>
              </g>
            );
          })}
          {area && <path d={area} fill="rgba(93, 112, 82, 0.1)" />}
          {line && (
            <path
              d={line}
              fill="none"
              stroke="#5D7052"
              strokeWidth="2.25"
              strokeLinecap="round"
              strokeLinejoin="round"
            />
          )}
          <line
            x1={pad.left}
            x2={width - pad.right}
            y1={thresholdY}
            y2={thresholdY}
            stroke="#C18C5D"
            strokeWidth="1.5"
            strokeDasharray="5 6"
          />
          <text
            x={width - pad.right}
            y={thresholdY - 6}
            textAnchor="end"
            fill="#C18C5D"
            fontSize="10"
            fontFamily="Nunito, sans-serif"
            fontWeight="700"
          >
            alert {money(threshold)}
          </text>
        </svg>
      </div>
    </div>
  );
}
