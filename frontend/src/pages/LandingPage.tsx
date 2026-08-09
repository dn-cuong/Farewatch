import { Link } from 'react-router-dom';
import { useAuth } from '../auth';
import styles from './LandingPage.module.css';

const preview = [
  {
    from: 'JFK',
    to: 'LAX',
    cities: 'New York → Los Angeles',
    detail: 'JetBlue · Fri Sep 18 · Nonstop',
    price: '$248',
    drop: '↓ $31 since yesterday',
  },
  {
    from: 'SFO',
    to: 'NRT',
    cities: 'San Francisco → Tokyo',
    detail: 'United · Oct 1 · 1 stop',
    price: '$742',
    drop: 'Watching under $680',
  },
  {
    from: 'BOS',
    to: 'LHR',
    cities: 'Boston → London',
    detail: 'British Airways · Aug 20',
    price: '$521',
    drop: '↓ 4.9% this week',
  },
];

export function LandingPage() {
  const { user } = useAuth();

  return (
    <div className="app-shell">
      <div className="blob blob-a" aria-hidden />
      <div className="blob blob-b" aria-hidden />
      <div className="blob blob-c" aria-hidden />

      <div className="app-content">
        <header className="topbar">
          <div className="brand">
            <span className="brand-mark">Fw</span>
            <div>
              <div className="brand-name">FareWatch</div>
              <div className="brand-tag">Flight fare alerts</div>
            </div>
          </div>
          <div className="topbar-actions">
            {user ? (
              <Link className="btn btn-primary" to="/dashboard">
                Dashboard
              </Link>
            ) : (
              <>
                <Link className="btn btn-ghost" to="/login">
                  Sign in
                </Link>
                <Link className="btn btn-primary" to="/search">
                  Get started
                </Link>
              </>
            )}
          </div>
        </header>

        <section className={styles.hero}>
          <div className={styles.copy}>
            <div className={styles.eyebrow}>Fare tracking</div>
            <h1>Watch a flight. Get emailed when it drops.</h1>
            <p className={styles.lead}>
              Search a route, pick the itinerary you want, and FareWatch will check prices and email
              you when it hits your target.
            </p>
            <div className={styles.cta}>
              <Link className="btn btn-primary" to="/search">
                {user ? 'Search flights' : 'Search a route'}
              </Link>
              <Link className="btn btn-outline" to={user ? '/dashboard' : '/login'}>
                {user ? 'Open my watches' : 'Sign in'}
              </Link>
            </div>
            <div className={styles.trust}>
              <span>Email alerts</span>
              <span>Only routes you choose</span>
              <span>Remove a watch anytime</span>
            </div>
          </div>

          <div className={styles.stage} aria-hidden>
            <div className={styles.stageGlow} />
            <div className={styles.cardStack}>
              {preview.map((card) => (
                <article className={styles.fareCard} key={`${card.from}-${card.to}`}>
                  <div className={styles.route}>
                    <div className={styles.codes}>
                      {card.from} → {card.to}
                    </div>
                    <div className={styles.price}>{card.price}</div>
                  </div>
                  <div className={styles.meta}>
                    {card.cities}
                    <br />
                    {card.detail}
                  </div>
                  <div className={styles.drop}>{card.drop}</div>
                </article>
              ))}
              <div className={styles.noteCard}>
                <strong>Example alert</strong>
                <p>“Your Boston → London fare dropped under $500.”</p>
              </div>
            </div>
          </div>
        </section>

        <section className={styles.section}>
          <h2>How it works</h2>
          <p className={styles.sectionLead}>Three steps.</p>
          <div className={styles.steps}>
            <article className={styles.step}>
              <div className={styles.stepNum}>1</div>
              <h3>Search</h3>
              <p>Compare airlines, flight numbers, times, and stops.</p>
            </article>
            <article className={styles.step}>
              <div className={styles.stepNum}>2</div>
              <h3>Watch</h3>
              <p>Save the itinerary you want and set a target price.</p>
            </article>
            <article className={styles.step}>
              <div className={styles.stepNum}>3</div>
              <h3>Get notified</h3>
              <p>When the fare drops under your target, you get an email.</p>
            </article>
          </div>
        </section>

        <section className={styles.band}>
          <div>
            <h2>Ready to track a fare?</h2>
            <p>Pick a route and set a number. We’ll email you when it drops.</p>
          </div>
          <Link className="btn btn-primary" to="/search">
            {user ? 'Search another flight' : 'Search a route'}
          </Link>
        </section>

        <footer className={styles.footer}>
          <span>FareWatch</span>
          <span>MIT</span>
        </footer>
      </div>
    </div>
  );
}
