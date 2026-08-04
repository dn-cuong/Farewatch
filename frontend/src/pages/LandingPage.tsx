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
              <div className="brand-tag">Quiet fare tracking</div>
            </div>
          </div>
          <div style={{ display: 'flex', gap: '0.65rem', paddingRight: '0.35rem' }}>
            {user ? (
              <Link className="btn btn-primary" to="/dashboard">
                Dashboard
              </Link>
            ) : (
              <>
                <Link className="btn btn-ghost" to="/login">
                  Sign in
                </Link>
                <Link className="btn btn-primary" to="/register">
                  Get started
                </Link>
              </>
            )}
          </div>
        </header>

        <section className={styles.hero}>
          <div className={styles.copy}>
            <div className={styles.eyebrow}>Flight fares, gently watched</div>
            <h1>Stop refreshing. Start being told.</h1>
            <p className={styles.lead}>
              FareWatch keeps an eye on the trips you actually care about — and emails you the moment
              a fare softens to your number. No noise. No dashboards you have to babysit.
            </p>
            <div className={styles.cta}>
              <Link className="btn btn-primary" to={user ? '/dashboard' : '/register'}>
                {user ? 'Open my watches' : 'Watch a route free'}
              </Link>
              <Link className="btn btn-outline" to="/login">
                I already have an account
              </Link>
            </div>
            <div className={styles.trust}>
              <span>Email alerts you control</span>
              <span>Only the routes you choose</span>
              <span>Cancel a watch anytime</span>
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
                <strong>A calm little ping</strong>
                <p>
                  “Your Boston → London fare dipped under $500.” That’s the whole product — the right
                  note, at the right time.
                </p>
              </div>
            </div>
          </div>
        </section>

        <section className={styles.section}>
          <h2>How it feels to use</h2>
          <p className={styles.sectionLead}>
            Three quiet steps. No hunting through airline apps every morning.
          </p>
          <div className={styles.steps}>
            <article className={styles.step}>
              <div className={styles.stepNum}>1</div>
              <h3>Tell us the trip</h3>
              <p>Pick where you’re flying, when, and the price that would make you book.</p>
            </article>
            <article className={styles.step}>
              <div className={styles.stepNum}>2</div>
              <h3>We keep watch</h3>
              <p>FareWatch checks your routes in the background while you get on with your day.</p>
            </article>
            <article className={styles.step}>
              <div className={styles.stepNum}>3</div>
              <h3>You get the email</h3>
              <p>When the fare crosses your line, a short note lands in your inbox — ready to act.</p>
            </article>
          </div>
        </section>

        <section className={styles.band}>
          <div>
            <h2>Built for people who hate fare FOMO</h2>
            <p>
              Whether it’s a long-planned Tokyo week or a last-minute hop home, set the number once.
              We’ll wait for the sky to settle — then nudge you.
            </p>
          </div>
          <Link className="btn btn-primary" to={user ? '/dashboard' : '/register'}>
            {user ? 'Go to dashboard' : 'Create free account'}
          </Link>
        </section>

        <footer className={styles.footer}>
          <span>FareWatch</span>
          <span>Watch less. Fly smarter.</span>
        </footer>
      </div>
    </div>
  );
}
