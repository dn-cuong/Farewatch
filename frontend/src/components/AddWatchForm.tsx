import { useEffect, useState, type FormEvent } from 'react';
import { createWatch, fetchAirports, type Airport } from '../api';
import styles from './AddWatchForm.module.css';

type Props = {
  onCreated: () => void;
};

export function AddWatchForm({ onCreated }: Props) {
  const [airports, setAirports] = useState<Airport[]>([]);
  const [origin, setOrigin] = useState('JFK');
  const [destination, setDestination] = useState('LAX');
  const [departDate, setDepartDate] = useState('2026-09-18');
  const [returnDate, setReturnDate] = useState('');
  const [threshold, setThreshold] = useState('275');
  const [busy, setBusy] = useState(false);
  const [notice, setNotice] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    fetchAirports()
      .then((list) => {
        const sorted = [...list].sort((a, b) => a.code.localeCompare(b.code));
        setAirports(sorted);
      })
      .catch(() => {
        setAirports([
          { code: 'JFK', city: 'New York', country: 'United States' },
          { code: 'LAX', city: 'Los Angeles', country: 'United States' },
          { code: 'SFO', city: 'San Francisco', country: 'United States' },
        ]);
      });
  }, []);

  async function submit(e: FormEvent) {
    e.preventDefault();
    if (origin === destination) {
      setError('Choose two different airports.');
      return;
    }
    setBusy(true);
    setError(null);
    try {
      const w = await createWatch({
        origin,
        destination,
        departDate,
        returnDate: returnDate || undefined,
        targetPrice: threshold ? Number(threshold) : undefined,
        dropPercent: 5,
        cabin: 'economy',
      });
      setNotice(`Watching ${w.route?.origin} → ${w.route?.destination}. Scanning airlines…`);
      onCreated();
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to create watch');
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className={`panel ${styles.panel}`}>
      <div className="panel-head">
        <h2>Notify me</h2>
        <span>Email alert</span>
      </div>
      <div className="panel-body">
        <form className="form-grid" onSubmit={submit}>
          <div className="field">
            <label htmlFor="origin">From</label>
            <select id="origin" className="mono" value={origin} onChange={(e) => setOrigin(e.target.value)}>
              {airports.map((a) => (
                <option key={a.code} value={a.code}>
                  {a.code} — {a.city}
                </option>
              ))}
            </select>
          </div>
          <div className="field">
            <label htmlFor="destination">To</label>
            <select
              id="destination"
              className="mono"
              value={destination}
              onChange={(e) => setDestination(e.target.value)}
            >
              {airports.map((a) => (
                <option key={a.code} value={a.code}>
                  {a.code} — {a.city}
                </option>
              ))}
            </select>
          </div>
          <div className="field">
            <label htmlFor="depart">Depart</label>
            <input
              id="depart"
              className="mono"
              type="date"
              value={departDate}
              onChange={(e) => setDepartDate(e.target.value)}
              required
            />
          </div>
          <div className="field">
            <label htmlFor="return">Return (optional)</label>
            <input
              id="return"
              className="mono"
              type="date"
              value={returnDate}
              onChange={(e) => setReturnDate(e.target.value)}
            />
          </div>
          <div className="field full">
            <label htmlFor="threshold">Email me under</label>
            <input
              id="threshold"
              className="mono"
              type="number"
              min={50}
              step={5}
              value={threshold}
              onChange={(e) => setThreshold(e.target.value)}
              required
            />
          </div>
          <div className="full">
            <button className="btn btn-primary" type="submit" disabled={busy} style={{ width: '100%' }}>
              {busy ? 'Saving…' : 'Save watch'}
            </button>
          </div>
        </form>

        {error && <p style={{ color: 'var(--fw-destructive)', marginTop: '0.85rem' }}>{error}</p>}
        {notice ? (
          <div className={styles.success}>
            <strong>Saved</strong> — {notice}
          </div>
        ) : (
          <p className={styles.hint}>
            Alerts go to your account email. We poll 12 airline APIs, cache in Redis, and store full
            offer details in Postgres.
          </p>
        )}
      </div>
    </div>
  );
}
