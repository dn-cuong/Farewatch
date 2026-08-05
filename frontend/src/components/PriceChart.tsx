import { useEffect, useMemo, useRef, useState } from 'react';
import { fetchFares, type Fare, type Watch } from '../api';
import { EmptyState, LoadingState } from './StatusStates';
import { formatPrice, formatPriceDelta } from '../utils/price';
import styles from './PriceChart.module.css';

type Props = {
  watch: Watch | null;
  onRemove?: (id: string) => void | Promise<void>;
  onToggleEmail?: (id: string, notifyOnDrop: boolean) => void | Promise<void>;
  busyAction?: string | null;
};

type RangeKey = '7d' | '30d' | 'all';

type Point = { date: string; price: number; at: string };

function formatDay(iso: string) {
  const d = new Date(`${iso.slice(0, 10)}T12:00:00`);
  if (Number.isNaN(d.getTime())) return iso.slice(5, 10);
  return d.toLocaleDateString(undefined, { month: 'short', day: 'numeric' });
}

function formatStamp(iso: string) {
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return formatDay(iso);
  return d.toLocaleString(undefined, {
    month: 'short',
    day: 'numeric',
    hour: 'numeric',
    minute: '2-digit',
  });
}

export function PriceChart({ watch, onRemove, onToggleEmail, busyAction }: Props) {
  const [fares, setFares] = useState<Fare[]>([]);
  const [loading, setLoading] = useState(false);
  const [range, setRange] = useState<RangeKey>('30d');
  const [hover, setHover] = useState<number | null>(null);
  const [confirmRemove, setConfirmRemove] = useState(false);
  const svgRef = useRef<SVGSVGElement>(null);

  useEffect(() => {
    setConfirmRemove(false);
    setHover(null);
  }, [watch?.id]);

  useEffect(() => {
    if (!watch?.routeId) {
      setFares([]);
      return;
    }
    setLoading(true);
    fetchFares(watch.routeId, 120)
      .then(setFares)
      .catch(() => setFares([]))
      .finally(() => setLoading(false));
  }, [watch?.routeId, watch?.latestFare?.observedAt]);

  const history = useMemo<Point[]>(() => {
    // Keep every scan observation for the selected flight. Collapsing to a
    // daily low hid all intraday movement and produced only one invisible
    // SVG point for newly-created watches.
    const matching = fares.filter(
      (fare) =>
        (!watch?.airlineCode || fare.airlineCode === watch.airlineCode) &&
        (!watch?.flightNumber || fare.flightNumber === watch.flightNumber),
    );
    const source = matching.length ? matching : fares;
    const byObservation = new Map<string, Point>();
    source.forEach((fare) => {
      byObservation.set(fare.observedAt, {
        date: fare.observedAt.slice(0, 10),
        price: fare.price,
        at: fare.observedAt,
      });
    });
    let points = [...byObservation.values()].sort(
      (a, b) => new Date(a.at).getTime() - new Date(b.at).getTime(),
    );
    if (range !== 'all' && points.length) {
      const days = range === '7d' ? 7 : 30;
      const cutoff = new Date();
      cutoff.setHours(0, 0, 0, 0);
      cutoff.setDate(cutoff.getDate() - (days - 1));
      points = points.filter((point) => new Date(point.at) >= cutoff);
    }
    if (!points.length && watch?.latestFare) {
      points = [
        {
          date: watch.latestFare.observedAt.slice(0, 10),
          price: watch.latestFare.price,
          at: watch.latestFare.observedAt,
        },
      ];
    }
    return points;
  }, [fares, range, watch?.airlineCode, watch?.flightNumber, watch?.latestFare]);

  if (!watch) {
    return (
      <div className={`panel ${styles.panelOrganic}`}>
        <div className="panel-head">
          <h2>Fare ticker</h2>
          <span>Select a watch</span>
        </div>
        <div className={styles.emptyPick}>
          <EmptyState
            title="Pick a route"
            message="Choose a watch on the left to open its price trail — like a stock chart for that flight."
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
  const first = history[0]?.price ?? latest?.price ?? threshold;
  const last = history[history.length - 1]?.price ?? latest?.price ?? threshold;
  const active = hover != null ? history[hover] : history[history.length - 1];
  const displayPrice = active?.price ?? last;
  const changeAbs = displayPrice - first;
  const changePct = first ? (changeAbs / first) * 100 : 0;
  const down = changeAbs < 0;
  const up = changeAbs > 0;
  const tone = down ? styles.toneDown : up ? styles.toneUp : styles.toneFlat;

  const width = 720;
  const height = 280;
  const pad = { top: 24, right: 18, bottom: 34, left: 46 };
  const prices = history.length ? history.map((p) => p.price) : [threshold];
  const min = Math.min(...prices, threshold) * 0.97;
  const max = Math.max(...prices, threshold) * 1.03;
  const innerW = width - pad.left - pad.right;
  const innerH = height - pad.top - pad.bottom;
  const x = (i: number) => pad.left + (i / Math.max(history.length - 1, 1)) * innerW;
  const y = (price: number) => pad.top + ((max - price) / (max - min || 1)) * innerH;
  const line =
    history.length === 1
      ? `M ${pad.left} ${y(history[0].price).toFixed(1)} L ${width - pad.right} ${y(history[0].price).toFixed(1)}`
      : history
          .map((p, i) => `${i === 0 ? 'M' : 'L'} ${x(i).toFixed(1)} ${y(p.price).toFixed(1)}`)
          .join(' ');
  const lastX = history.length === 1 ? width - pad.right : x(history.length - 1);
  const area = history.length
    ? `${line} L ${lastX.toFixed(1)} ${pad.top + innerH} L ${pad.left} ${pad.top + innerH} Z`
    : '';
  const thresholdY = y(threshold);
  const stroke = down ? 'var(--fw-down)' : up ? 'var(--fw-up)' : 'var(--fw-primary)';
  const fill = down
    ? 'rgba(93, 112, 82, 0.16)'
    : up
      ? 'rgba(168, 84, 72, 0.12)'
      : 'rgba(93, 112, 82, 0.1)';

  function onPointer(clientX: number) {
    const svg = svgRef.current;
    if (!svg || !history.length) return;
    if (history.length === 1) {
      setHover(0);
      return;
    }
    const rect = svg.getBoundingClientRect();
    const ratio = (clientX - rect.left) / rect.width;
    const px = ratio * width;
    const idx = Math.round(((px - pad.left) / innerW) * (history.length - 1));
    setHover(Math.max(0, Math.min(history.length - 1, idx)));
  }

  const emailOn = watch.notifyOnDrop;
  const removing = busyAction === `remove:${watch.id}`;
  const toggling = busyAction === `email:${watch.id}`;

  return (
    <div className={`panel ${styles.panelOrganic}`}>
      <div className="panel-head">
        <h2>
          {watch.route?.origin} → {watch.route?.destination}
        </h2>
        <span className="mono">{watch.flightNumber || latest?.flightNumber || 'SCAN'}</span>
      </div>

      <div className={`panel-body ${styles.wrap}`}>
        <div className={styles.tickerHead}>
          <div>
            <div className={styles.symbol}>
              {(watch.route?.origin ?? '???')}-{(watch.route?.destination ?? '???')}
              {watch.flightNumber ? ` · ${watch.flightNumber}` : ''}
            </div>
            <div className={styles.priceRow}>
              <strong className={styles.bigPrice}>{formatPrice(displayPrice, latest?.currency ?? 'USD')}</strong>
              <span className={`${styles.delta} ${tone}`}>
                {changeAbs === 0 ? '0.0%' : `${changeAbs > 0 ? '+' : ''}${changePct.toFixed(1)}%`}
                <small>
                  {changeAbs === 0 ? 'flat' : formatPriceDelta(changeAbs, latest?.currency ?? 'USD')}
                </small>
              </span>
            </div>
            <p className={styles.hoverHint}>
              {hover != null && active
                ? formatStamp(active.at)
                : latest
                  ? `${latest.airline} · last scan ${formatStamp(latest.observedAt)}`
                  : 'Waiting for first scan'}
            </p>
          </div>

          <div className={styles.ranges} role="group" aria-label="Chart range">
            {(
              [
                ['7d', '7D'],
                ['30d', '30D'],
                ['all', 'All'],
              ] as const
            ).map(([key, label]) => (
              <button
                key={key}
                type="button"
                className={range === key ? styles.rangeActive : undefined}
                onClick={() => setRange(key)}
              >
                {label}
              </button>
            ))}
          </div>
        </div>

        <div className={styles.metrics}>
          <div className={styles.metric}>
            <label>Low</label>
            <strong>{formatPrice(Math.min(...prices), latest?.currency ?? 'USD')}</strong>
          </div>
          <div className={styles.metric}>
            <label>High</label>
            <strong>{formatPrice(Math.max(...prices), latest?.currency ?? 'USD')}</strong>
          </div>
          <div className={styles.metric}>
            <label>Alert ≤</label>
            <strong className={styles.alertTone}>{formatPrice(threshold, latest?.currency ?? 'USD')}</strong>
          </div>
          <div className={styles.metric}>
            <label>Email</label>
            <strong>{emailOn ? 'On' : 'Muted'}</strong>
          </div>
        </div>

        <div
          className={styles.chartShell}
          onPointerLeave={() => setHover(null)}
        >
          <svg
            ref={svgRef}
            className={styles.chart}
            viewBox={`0 0 ${width} ${height}`}
            role="img"
            aria-label="Fare price history"
            onPointerMove={(e) => onPointer(e.clientX)}
            onPointerDown={(e) => onPointer(e.clientX)}
          >
            {[0, 0.25, 0.5, 0.75, 1].map((t) => {
              const gy = pad.top + innerH * t;
              const label = Math.round(max - (max - min) * t);
              return (
                <g key={t}>
                  <line
                    x1={pad.left}
                    x2={width - pad.right}
                    y1={gy}
                    y2={gy}
                    stroke="rgba(222, 216, 207, 0.75)"
                  />
                  <text x={8} y={gy + 4} fill="#78786C" fontSize="11" fontFamily="Nunito, sans-serif">
                    {label}
                  </text>
                </g>
              );
            })}

            {area && <path d={area} fill={fill} />}
            {line && (
              <path
                d={line}
                fill="none"
                stroke={stroke}
                strokeWidth="2.6"
                strokeLinecap="round"
                strokeLinejoin="round"
              />
            )}
            {history.length > 0 && (
              <circle
                cx={history.length === 1 ? width - pad.right : x(history.length - 1)}
                cy={y(history[history.length - 1].price)}
                r="5"
                fill="#FDFCF8"
                stroke={stroke}
                strokeWidth="2.75"
              />
            )}

            <line
              x1={pad.left}
              x2={width - pad.right}
              y1={thresholdY}
              y2={thresholdY}
              stroke="#C18C5D"
              strokeWidth="1.6"
              strokeDasharray="5 6"
            />
            <text
              x={width - pad.right}
              y={thresholdY - 8}
              textAnchor="end"
              fill="#C18C5D"
              fontSize="11"
              fontFamily="Nunito, sans-serif"
              fontWeight="800"
            >
              alert {formatPrice(threshold, latest?.currency ?? 'USD')}
            </text>

            {history.length > 1 &&
              [0, Math.floor((history.length - 1) / 2), history.length - 1].map((i) => (
                <text
                  key={`${history[i].date}-${i}`}
                  x={x(i)}
                  y={height - 10}
                  textAnchor={i === 0 ? 'start' : i === history.length - 1 ? 'end' : 'middle'}
                  fill="#78786C"
                  fontSize="11"
                  fontFamily="Nunito, sans-serif"
                >
                  {formatDay(history[i].date)}
                </text>
              ))}

            {hover != null && history[hover] && (
              <g>
                <line
                  x1={x(hover)}
                  x2={x(hover)}
                  y1={pad.top}
                  y2={pad.top + innerH}
                  stroke="rgba(44, 44, 36, 0.28)"
                  strokeDasharray="3 4"
                />
                <circle
                  cx={x(hover)}
                  cy={y(history[hover].price)}
                  r="5.5"
                  fill="#FDFCF8"
                  stroke={stroke}
                  strokeWidth="2.5"
                />
              </g>
            )}
          </svg>

          {hover != null && active && (
            <div
              className={styles.tooltip}
              style={{
                left: `${(x(hover) / width) * 100}%`,
              }}
            >
              <strong>{formatPrice(active.price, latest?.currency ?? 'USD')}</strong>
              <span>{formatDay(active.date)}</span>
            </div>
          )}
        </div>

        {latest && (
          <p className={styles.flightMeta}>
            {latest.flightNumber} · {latest.airline} · {latest.stops === 0 ? 'Nonstop' : `${latest.stops} stop`}
            {latest.aircraft ? ` · ${latest.aircraft}` : ''} · departs {formatStamp(latest.departAt)}
          </p>
        )}

        <div className={styles.actions}>
          {onToggleEmail && (
            <button
              className="btn btn-outline"
              type="button"
              disabled={toggling || removing}
              onClick={() => onToggleEmail(watch.id, !emailOn)}
            >
              {toggling ? 'Updating…' : emailOn ? 'Mute email alerts' : 'Turn email alerts back on'}
            </button>
          )}
          {onRemove && !confirmRemove && (
            <button
              className={`btn btn-ghost ${styles.removeBtn}`}
              type="button"
              disabled={removing || toggling}
              onClick={() => setConfirmRemove(true)}
            >
              Remove from watchlist
            </button>
          )}
          {onRemove && confirmRemove && (
            <div className={styles.confirm}>
              <p>Stop watching this route and stop all alerts for it?</p>
              <button
                className="btn btn-primary"
                type="button"
                disabled={removing}
                onClick={() => onRemove(watch.id)}
              >
                {removing ? 'Removing…' : 'Yes, remove'}
              </button>
              <button className="btn btn-ghost" type="button" onClick={() => setConfirmRemove(false)}>
                Keep watching
              </button>
            </div>
          )}
        </div>

        <p className={styles.emailNote}>
          {emailOn
            ? `Alerts go to ${watch.email} when the fare hits ≤ ${formatPrice(threshold, latest?.currency ?? 'USD')}.`
            : `Watch stays on the board, but ${watch.email} will not get drop emails until you turn alerts back on.`}
        </p>
      </div>
    </div>
  );
}
