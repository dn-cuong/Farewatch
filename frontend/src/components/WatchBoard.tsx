import type { Watch } from '../api';
import { EmptyState, ErrorState, LoadingState } from './StatusStates';
import { formatPrice } from '../utils/price';
import styles from './WatchBoard.module.css';

type Props = {
  watches: Watch[];
  selectedId: string | null;
  onSelect: (id: string) => void;
  onRemove?: (id: string) => void;
  status?: 'ready' | 'loading' | 'error' | 'empty';
  onRetry?: () => void;
};

function changeClass(change?: number | null) {
  if (change == null) return styles.flat;
  if (change < 0) return styles.down;
  if (change > 0) return styles.up;
  return styles.flat;
}

function formatChange(change?: number | null) {
  if (change == null) return '—';
  if (change === 0) return '0.0%';
  const sign = change > 0 ? '+' : '';
  return `${sign}${change.toFixed(1)}%`;
}

function formatTime(iso?: string) {
  if (!iso) return '—';
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return '—';
  return d.toLocaleString(undefined, {
    month: 'short',
    day: 'numeric',
    hour: '2-digit',
    minute: '2-digit',
  });
}

function durationLabel(mins?: number) {
  if (!mins) return '—';
  const h = Math.floor(mins / 60);
  const m = mins % 60;
  return `${h}h ${m.toString().padStart(2, '0')}m`;
}

export function WatchBoard({ watches, selectedId, onSelect, onRemove, status = 'ready', onRetry }: Props) {
  if (status === 'loading') {
    return (
      <div className={`panel ${styles.board}`}>
        <div className="panel-head">
          <h2>Your watches</h2>
          <span>Gathering</span>
        </div>
        <LoadingState title="Loading dashboard" message="Fetching your notified routes…" />
      </div>
    );
  }

  if (status === 'error') {
    return (
      <div className={`panel ${styles.board}`}>
        <div className="panel-head">
          <h2>Your watches</h2>
          <span>Paused</span>
        </div>
        <ErrorState
          title="Couldn’t load watches"
          message="Check that the API is up, then try again."
          action={
            onRetry ? (
              <button className="btn btn-outline" type="button" onClick={onRetry}>
                Try again
              </button>
            ) : null
          }
        />
      </div>
    );
  }

  if (status === 'empty' || !watches.length) {
    return (
      <div className={`panel ${styles.board}`}>
        <div className="panel-head">
          <h2>Your watches</h2>
          <span>Empty</span>
        </div>
        <EmptyState
          title="No email watches yet"
          message="Search a route, pick a flight, and we’ll email you when that fare drops."
        />
      </div>
    );
  }

  return (
    <div className={`panel ${styles.board}`}>
      <div className="panel-head">
        <h2>Your watches</h2>
        <span className="mono">{watches.length} notifying</span>
      </div>

      <div className={styles.desktop}>
        <table className={styles.table}>
          <thead>
            <tr>
              <th>Route</th>
              <th>Flight</th>
              <th>Schedule</th>
              <th>Fare</th>
              <th>24h</th>
              <th>Status</th>
              <th></th>
            </tr>
          </thead>
          <tbody>
            {watches.map((w) => {
              const f = w.latestFare;
              const alerted = f && w.targetPrice != null ? f.price <= w.targetPrice : false;
              const muted = !w.notifyOnDrop;
              return (
                <tr
                  key={w.id}
                  className={`${styles.row} ${selectedId === w.id ? styles.rowActive : ''}`}
                  onClick={() => onSelect(w.id)}
                >
                  <td>
                    <div className={styles.routeCodes}>
                      <span>{w.route?.origin ?? f?.origin}</span>
                      <span className={styles.arrow}>→</span>
                      <span>{w.route?.destination ?? f?.destination}</span>
                    </div>
                    <div className={styles.cities}>
                      {f ? `${f.originCity} — ${f.destinationCity}` : w.route?.departDate}
                      {w.targetPrice != null ? ` · alert ≤ ${formatPrice(w.targetPrice, f?.currency ?? 'USD')}` : ''}
                    </div>
                  </td>
                  <td>
                    <div className={styles.flight}>{f?.flightNumber ?? w.flightNumber ?? 'Scanning…'}</div>
                    <div className={styles.meta}>
                      {f
                        ? `${f.airline} · ${f.aircraft} · ${f.stops === 0 ? 'Nonstop' : `${f.stops} stop`}`
                        : w.airlineCode
                          ? `${w.airlineCode}${w.targetPrice != null ? ` · alert ≤ ${formatPrice(w.targetPrice)}` : ''}`
                          : '—'}
                    </div>
                  </td>
                  <td>
                    <div className={styles.meta}>{formatTime(f?.departAt)}</div>
                    <div className={styles.meta}>
                      {durationLabel(f?.durationMinutes)} · arrives {formatTime(f?.arriveAt)}
                    </div>
                  </td>
                  <td>
                    {f ? <span className={styles.price}>{formatPrice(f.price, f.currency)}</span> : <span className={styles.meta}>…</span>}
                  </td>
                  <td>
                    <span className={`${styles.change} ${changeClass(w.change24h)}`}>
                      {formatChange(w.change24h)}
                    </span>
                  </td>
                  <td>
                    <span className={`${styles.badge} ${alerted ? styles.badgeAlert : ''} ${muted ? styles.badgeMuted : ''}`}>
                      {muted ? 'Email muted' : alerted ? 'Alert ready' : 'Watching'}
                    </span>
                  </td>
                  <td>
                    {onRemove && (
                      <button
                        className={`btn btn-ghost ${styles.remove}`}
                        type="button"
                        onClick={(e) => {
                          e.stopPropagation();
                          onRemove(w.id);
                        }}
                      >
                        Remove
                      </button>
                    )}
                  </td>
                </tr>
              );
            })}
          </tbody>
        </table>
      </div>

      <div className={styles.cards}>
        {watches.map((w) => {
          const f = w.latestFare;
          const alerted = f && w.targetPrice != null ? f.price <= w.targetPrice : false;
          const muted = !w.notifyOnDrop;
          return (
            <div
              key={w.id}
              className={`${styles.card} ${selectedId === w.id ? styles.cardActive : ''}`}
              onClick={() => onSelect(w.id)}
              role="button"
              tabIndex={0}
              onKeyDown={(e) => e.key === 'Enter' && onSelect(w.id)}
            >
              <div className={styles.cardTop}>
                <div>
                  <div className={styles.routeCodes}>
                    <span>{w.route?.origin}</span>
                    <span className={styles.arrow}>→</span>
                    <span>{w.route?.destination}</span>
                  </div>
                  <div className={styles.cities}>
                    {f?.flightNumber ?? 'Scanning…'} · {f?.airline ?? '—'}
                  </div>
                </div>
                  {f ? <span className={styles.price}>{formatPrice(f.price, f.currency)}</span> : null}
              </div>
              <div className={styles.cardBottom}>
                <div>
                  <div className={styles.meta}>{formatTime(f?.departAt)}</div>
                  <span className={`${styles.change} ${changeClass(w.change24h)}`}>
                    {formatChange(w.change24h)}
                  </span>
                </div>
                <div className={styles.cardActions}>
                  <span className={`${styles.badge} ${alerted ? styles.badgeAlert : ''} ${muted ? styles.badgeMuted : ''}`}>
                    {muted ? 'Email muted' : alerted ? 'Alert ready' : 'Watching'}
                  </span>
                  {onRemove && (
                    <button
                      className={`btn btn-ghost ${styles.remove}`}
                      type="button"
                      onClick={(e) => {
                        e.stopPropagation();
                        onRemove(w.id);
                      }}
                    >
                      Remove
                    </button>
                  )}
                </div>
              </div>
            </div>
          );
        })}
      </div>
    </div>
  );
}
