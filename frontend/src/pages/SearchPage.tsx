import { useEffect, useMemo, useState, type FormEvent } from 'react';
import { Link, useNavigate } from 'react-router-dom';
import {
  createEmailWatch,
  createWatch,
  fetchAirports,
  fetchBookingLinks,
  searchFares,
  type Airport,
  type BookingLink,
  type FlightOffer,
} from '../api';
import { useAuth } from '../auth';
import { CalendarPicker } from '../components/CalendarPicker';
import { FlightDropdown, type FlightDropdownOption } from '../components/FlightDropdown';
import { savePendingWatch } from '../pendingWatch';
import { formatPrice } from '../utils/price';
import styles from './SearchPage.module.css';

type TripType = 'oneway' | 'roundtrip';

function formatClock(iso: string) {
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return '—';
  return d.toLocaleTimeString(undefined, { hour: 'numeric', minute: '2-digit' });
}

function formatDay(iso: string) {
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return '—';
  return d.toLocaleDateString(undefined, { weekday: 'short', month: 'short', day: 'numeric' });
}

function formatDuration(mins: number) {
  const h = Math.floor(mins / 60);
  const m = mins % 60;
  if (h <= 0) return `${m}m`;
  return `${h}h ${m.toString().padStart(2, '0')}m`;
}

function stopsLabel(stops: number, layovers: string[]) {
  if (stops <= 0) return 'Nonstop';
  if (layovers.length) return `${stops} stop · via ${layovers.join(', ')}`;
  return `${stops} stop${stops > 1 ? 's' : ''}`;
}

function defaultDepartDate() {
  const d = new Date();
  d.setDate(d.getDate() + 21);
  return localDateValue(d);
}

function defaultReturnDate(depart: string) {
  const d = new Date(`${depart}T12:00:00`);
  if (Number.isNaN(d.getTime())) return '';
  d.setDate(d.getDate() + 7);
  return localDateValue(d);
}

function localDateValue(date: Date) {
  const year = date.getFullYear();
  const month = String(date.getMonth() + 1).padStart(2, '0');
  const day = String(date.getDate()).padStart(2, '0');
  return `${year}-${month}-${day}`;
}

export function SearchPage() {
  const { user } = useAuth();
  const navigate = useNavigate();
  const [airports, setAirports] = useState<Airport[]>([]);
  const [origin, setOrigin] = useState('JFK');
  const [destination, setDestination] = useState('LAX');
  const [tripType, setTripType] = useState<TripType>('oneway');
  const [departDate, setDepartDate] = useState(defaultDepartDate);
  const [returnDate, setReturnDate] = useState('');
  const [cabin, setCabin] = useState('economy');
  const [offers, setOffers] = useState<FlightOffer[]>([]);
  const [searched, setSearched] = useState(false);
  const [busy, setBusy] = useState(false);
  const [selecting, setSelecting] = useState<string | null>(null);
  const [selectedOffer, setSelectedOffer] = useState<FlightOffer | null>(null);
  const [email, setEmail] = useState('');
  const [emailBusy, setEmailBusy] = useState(false);
  const [emailSaved, setEmailSaved] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [bookingOffer, setBookingOffer] = useState<FlightOffer | null>(null);
  const [bookingLinks, setBookingLinks] = useState<BookingLink[]>([]);
  const [bookingBusy, setBookingBusy] = useState(false);
  const [bookingError, setBookingError] = useState<string | null>(null);

  useEffect(() => {
    fetchAirports()
      .then(setAirports)
      .catch(() =>
        setAirports([
          { code: 'JFK', city: 'New York', country: 'United States' },
          { code: 'LAX', city: 'Los Angeles', country: 'United States' },
          { code: 'SFO', city: 'San Francisco', country: 'United States' },
        ]),
      );
  }, []);

  const airportOptions = useMemo<FlightDropdownOption[]>(() => {
    return airports.map((airport) => ({
      value: airport.code,
      code: airport.code,
      label: airport.city,
      group: airport.country || 'Other',
      keywords: `${airport.code} ${airport.city} ${airport.country}`,
    }));
  }, [airports]);

  const cabinOptions = useMemo<FlightDropdownOption[]>(
    () => [
      { value: 'economy', label: 'Economy' },
      { value: 'premium_economy', label: 'Premium economy' },
      { value: 'business', label: 'Business' },
    ],
    [],
  );

  const title = useMemo(() => {
    if (!searched) return 'Find a flight to watch';
    const arrow = tripType === 'roundtrip' ? '⇄' : '→';
    return `${origin} ${arrow} ${destination} · ${offers.length} option${offers.length === 1 ? '' : 's'}`;
  }, [searched, origin, destination, offers.length, tripType]);

  function setTrip(next: TripType) {
    setTripType(next);
    if (next === 'roundtrip' && !returnDate) {
      setReturnDate(defaultReturnDate(departDate));
    }
    if (next === 'oneway') {
      setReturnDate('');
    }
  }

  function swapAirports() {
    setOrigin(destination);
    setDestination(origin);
  }

  async function onSearch(e: FormEvent) {
    e.preventDefault();
    if (origin === destination) {
      setError('Pick two different airports.');
      return;
    }
    if (tripType === 'roundtrip' && !returnDate) {
      setError('Pick a return date for a round-trip search.');
      return;
    }
    if (tripType === 'roundtrip' && returnDate < departDate) {
      setError('Return date must be on or after the depart date.');
      return;
    }

    setBusy(true);
    setError(null);
    setSearched(true);
    try {
      const list = await searchFares({
        origin,
        destination,
        departDate,
        returnDate: tripType === 'roundtrip' ? returnDate : undefined,
        cabin,
      });
      setOffers(list);
    } catch (err) {
      setOffers([]);
      setError(err instanceof Error ? err.message : 'Search failed');
    } finally {
      setBusy(false);
    }
  }

  async function onSelect(offer: FlightOffer) {
    const target = Math.max(50, Math.round(offer.price * 0.95 * 100) / 100);
    const pending = {
      origin: offer.origin,
      destination: offer.destination,
      departDate,
      returnDate: tripType === 'roundtrip' ? returnDate || undefined : undefined,
      cabin: offer.cabin || cabin,
      airlineCode: offer.airlineCode,
      flightNumber: offer.flightNumber,
      airline: offer.airline,
      price: offer.price,
      currency: offer.currency,
      targetPrice: target,
      stops: offer.stops,
      layoverAirports: offer.layoverAirports ?? [],
      departAt: offer.departAt,
      arriveAt: offer.arriveAt,
    };

    if (!user) {
      setSelectedOffer(offer);
      setEmailSaved(false);
      return;
    }

    setSelecting(offer.offerId || offer.flightNumber);
    setError(null);
    try {
      await createWatch({
        origin: pending.origin,
        destination: pending.destination,
        departDate: pending.departDate,
        returnDate: pending.returnDate,
        cabin: pending.cabin,
        airlineCode: pending.airlineCode,
        flightNumber: pending.flightNumber,
        targetPrice: pending.targetPrice,
        dropPercent: 5,
      });
      navigate('/dashboard');
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Could not start watch');
    } finally {
      setSelecting(null);
    }
  }

  function continueWithAccount(path = '/register?next=watch') {
    if (!selectedOffer) return;
    savePendingWatch({
      origin: selectedOffer.origin,
      destination: selectedOffer.destination,
      departDate,
      returnDate: tripType === 'roundtrip' ? returnDate || undefined : undefined,
      cabin: selectedOffer.cabin || cabin,
      airlineCode: selectedOffer.airlineCode,
      flightNumber: selectedOffer.flightNumber,
      airline: selectedOffer.airline,
      price: selectedOffer.price,
      currency: selectedOffer.currency,
      targetPrice: Math.max(50, Math.round(selectedOffer.price * 0.95 * 100) / 100),
      stops: selectedOffer.stops,
      layoverAirports: selectedOffer.layoverAirports ?? [],
      departAt: selectedOffer.departAt,
      arriveAt: selectedOffer.arriveAt,
    });
    navigate(path);
  }

  async function submitEmailWatch(e: FormEvent) {
    e.preventDefault();
    if (!selectedOffer) return;
    setEmailBusy(true);
    setError(null);
    try {
      await createEmailWatch({
        email,
        origin: selectedOffer.origin,
        destination: selectedOffer.destination,
        departDate,
        returnDate: tripType === 'roundtrip' ? returnDate || undefined : undefined,
        cabin: selectedOffer.cabin || cabin,
        airlineCode: selectedOffer.airlineCode,
        flightNumber: selectedOffer.flightNumber,
        targetPrice: Math.max(50, Math.round(selectedOffer.price * 0.95 * 100) / 100),
      });
      setEmailSaved(true);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Could not create email alert');
    } finally {
      setEmailBusy(false);
    }
  }

  async function openBooking(offer: FlightOffer) {
    setBookingOffer(offer);
    setBookingLinks([]);
    setBookingError(null);
    setBookingBusy(true);
    try {
      const links = await fetchBookingLinks({
        offerId: offer.offerId || offer.flightNumber,
        origin: offer.origin,
        destination: offer.destination,
        departDate,
        returnDate: tripType === 'roundtrip' ? returnDate || undefined : undefined,
      });
      setBookingLinks(links);
    } catch (err) {
      setBookingError(err instanceof Error ? err.message : 'Could not load booking links');
      if (offer.deepLink) {
        setBookingLinks([
          {
            providerName: 'Google Flights',
            providerType: 'metasearch',
            fareName: '',
            price: 0,
            currency: offer.currency,
            url: offer.deepLink,
          },
        ]);
      }
    } finally {
      setBookingBusy(false);
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
            <div>
              <div className="brand-name">FareWatch</div>
              <div className="brand-tag">Find a flight</div>
            </div>
          </Link>
          <div className={`topbar-actions ${styles.navActions}`}>
            {user ? (
              <Link className="btn btn-ghost" to="/dashboard">
                Dashboard
              </Link>
            ) : (
              <>
                <Link className="btn btn-ghost" to="/login">
                  Sign in
                </Link>
                <Link className="btn btn-primary" to="/register">
                  Register
                </Link>
              </>
            )}
          </div>
        </header>

        <section className={styles.hero}>
          <div className={styles.heroCopy}>
            <p className={styles.eyebrow}>Live fares · multi-airline</p>
            <h1>{title}</h1>
            <p className={styles.lead}>
              Choose one-way or round-trip, compare real itineraries, then track the exact flight you
              care about — or jump out to book with an airline or Google Flights.
            </p>
          </div>

          <form className={styles.search} onSubmit={onSearch}>
            <div className={styles.tripRow}>
              <div className={styles.tripToggle} role="group" aria-label="Trip type">
                <button
                  type="button"
                  className={tripType === 'oneway' ? styles.tripActive : undefined}
                  aria-pressed={tripType === 'oneway'}
                  onClick={() => setTrip('oneway')}
                >
                  <span className={styles.tripCheck} aria-hidden="true">
                    {tripType === 'oneway' ? '✓' : ''}
                  </span>
                  One-way
                </button>
                <button
                  type="button"
                  className={tripType === 'roundtrip' ? styles.tripActive : undefined}
                  aria-pressed={tripType === 'roundtrip'}
                  onClick={() => setTrip('roundtrip')}
                >
                  <span className={styles.tripCheck} aria-hidden="true">
                    {tripType === 'roundtrip' ? '✓' : ''}
                  </span>
                  Round-trip
                </button>
              </div>
              <p className={styles.tripHint}>
                {tripType === 'roundtrip'
                  ? 'We’ll search fares that include a return date.'
                  : 'We’ll search outbound-only fares.'}
              </p>
            </div>

            <div className={styles.fields}>
              <div className={`field ${styles.routeField} ${styles.fromField}`}>
                <label htmlFor="origin">From</label>
                <FlightDropdown
                  id="origin"
                  value={origin}
                  options={airportOptions}
                  onChange={setOrigin}
                  searchable
                  searchPlaceholder="Search city, airport, or country"
                />
              </div>

              <button className={styles.swap} type="button" onClick={swapAirports} aria-label="Swap airports">
                ⇄
              </button>

              <div className={`field ${styles.routeField} ${styles.toField}`}>
                <label htmlFor="destination">To</label>
                <FlightDropdown
                  id="destination"
                  value={destination}
                  options={airportOptions}
                  onChange={setDestination}
                  searchable
                  searchPlaceholder="Search city, airport, or country"
                />
              </div>

              <div className={`field ${styles.departField}`}>
                <label htmlFor="depart">Depart</label>
                <CalendarPicker
                  id="depart"
                  value={departDate}
                  min={localDateValue(new Date())}
                  onChange={(value) => {
                    setDepartDate(value);
                    if (tripType === 'roundtrip' && returnDate && returnDate < value) {
                      setReturnDate(value);
                    }
                  }}
                />
              </div>

              <div className={`field ${styles.returnField} ${tripType === 'oneway' ? styles.returnDisabled : ''}`}>
                <label htmlFor="return">Return</label>
                <CalendarPicker
                  id="return"
                  value={returnDate}
                  min={departDate}
                  disabled={tripType === 'oneway'}
                  onChange={setReturnDate}
                  placeholder={tripType === 'oneway' ? 'One-way' : 'Select date'}
                />
              </div>

              <div className={`field ${styles.cabinField}`}>
                <label htmlFor="cabin">Cabin</label>
                <FlightDropdown
                  id="cabin"
                  value={cabin}
                  options={cabinOptions}
                  onChange={setCabin}
                />
              </div>
            </div>

            <div className={styles.searchFooter}>
              <p className={styles.searchNote}>
                {tripType === 'roundtrip' ? `Round-trip · return ${returnDate || '—'}` : 'One-way trip'}
              </p>
              <button className="btn btn-primary" type="submit" disabled={busy}>
                {busy ? 'Searching…' : 'Search flights'}
              </button>
            </div>
          </form>
        </section>

        {error && !selectedOffer && !bookingOffer && <p className={styles.error}>{error}</p>}

        {searched && !busy && offers.length === 0 && !error && (
          <p className={styles.empty}>No flights found for that search. Try another date or route.</p>
        )}

        {busy && <p className={styles.empty}>Checking live fares…</p>}

        <div className={styles.list}>
          {offers.map((offer) => {
            const key = offer.offerId || `${offer.airlineCode}-${offer.flightNumber}-${offer.departAt}`;
            return (
              <article className={styles.card} key={key}>
                <div className={styles.cardTop}>
                  <div>
                    <div className={styles.airline}>
                      {offer.airline} · <span className="mono">{offer.flightNumber}</span>
                    </div>
                    <div className={styles.cities}>
                      {offer.originCity} → {offer.destinationCity}
                      {tripType === 'roundtrip' ? ' · Round-trip' : ' · One-way'}
                    </div>
                  </div>
                  <div className={styles.priceBlock}>
                    <div className={styles.price}>{formatPrice(offer.price, offer.currency)}</div>
                    <div className={styles.priceNote}>{tripType === 'roundtrip' ? 'total fare' : 'one-way'}</div>
                  </div>
                </div>

                <div className={styles.timeline}>
                  <div className={styles.timeCol}>
                    <strong>{formatClock(offer.departAt)}</strong>
                    <span className="mono">{offer.origin}</span>
                    <span>{formatDay(offer.departAt)}</span>
                  </div>
                  <div className={styles.timeMid}>
                    <span>{formatDuration(offer.durationMinutes)}</span>
                    <div className={styles.timeLine} />
                    <span>{stopsLabel(offer.stops, offer.layoverAirports ?? [])}</span>
                  </div>
                  <div className={styles.timeCol}>
                    <strong>{formatClock(offer.arriveAt)}</strong>
                    <span className="mono">{offer.destination}</span>
                    <span>{formatDay(offer.arriveAt)}</span>
                  </div>
                </div>

                {offer.segments?.length > 0 && (
                  <details className={styles.details}>
                    <summary>Flight details · {offer.segments.length} leg{offer.segments.length > 1 ? 's' : ''}</summary>
                    <div className={styles.segments}>
                      {offer.segments.map((seg, idx) => (
                        <div className={styles.segment} key={`${seg.flightNumber}-${idx}`}>
                          <div className={styles.segHead}>
                            <strong className="mono">{seg.flightNumber}</strong>
                            <span>
                              {seg.origin} → {seg.destination}
                            </span>
                            <span>{formatDuration(seg.durationMinutes)}</span>
                          </div>
                          <div className={styles.segTimes}>
                            {formatClock(seg.departAt)} – {formatClock(seg.arriveAt)}
                            {seg.aircraft ? ` · ${seg.aircraft}` : ''}
                          </div>
                          {idx < offer.segments.length - 1 && (
                            <div className={styles.layover}>
                              Layover in {seg.destinationCity} ({seg.destination})
                            </div>
                          )}
                        </div>
                      ))}
                    </div>
                  </details>
                )}

                <div className={styles.cardActions}>
                  <button className="btn btn-primary" type="button" disabled={!!selecting} onClick={() => onSelect(offer)}>
                    {selecting === (offer.offerId || offer.flightNumber)
                      ? 'Saving…'
                      : user
                        ? 'Watch this flight'
                        : 'Track this flight'}
                  </button>
                  <button className="btn btn-outline" type="button" onClick={() => openBooking(offer)}>
                    Book options
                  </button>
                </div>
              </article>
            );
          })}
        </div>

        {selectedOffer && (
          <div className={styles.modalBackdrop} role="presentation" onMouseDown={() => setSelectedOffer(null)}>
            <section
              className={styles.modal}
              role="dialog"
              aria-modal="true"
              aria-labelledby="watch-choice-title"
              onMouseDown={(e) => e.stopPropagation()}
            >
              <button className={styles.close} type="button" aria-label="Close" onClick={() => setSelectedOffer(null)}>
                ×
              </button>
              <p className={styles.eyebrow}>Your selected flight</p>
              <h2 id="watch-choice-title">
                {selectedOffer.airline} {selectedOffer.flightNumber}
              </h2>
              <p className={styles.modalRoute}>
                {selectedOffer.origin} → {selectedOffer.destination} · {formatPrice(selectedOffer.price, selectedOffer.currency)}
                {tripType === 'roundtrip' ? ' · Round-trip' : ' · One-way'}
              </p>
              {error && <p className={styles.modalError}>{error}</p>}

              {emailSaved ? (
                <div className={styles.success}>
                  <strong>You’re all set.</strong>
                  <p>We’ll email {email} when this fare drops below your target.</p>
                  <button className="btn btn-primary" type="button" onClick={() => setSelectedOffer(null)}>
                    Done
                  </button>
                </div>
              ) : (
                <div className={styles.choices}>
                  <div className={styles.choice}>
                    <h3>Use a dashboard</h3>
                    <p>Register to manage watches, view price history, and change alerts later.</p>
                    <button className="btn btn-primary" type="button" onClick={() => continueWithAccount()}>
                      Create account
                    </button>
                    <button
                      className={styles.textButton}
                      type="button"
                      onClick={() => continueWithAccount('/login?next=watch')}
                    >
                      I already have an account
                    </button>
                  </div>

                  <form className={styles.choice} onSubmit={submitEmailWatch}>
                    <h3>Email only</h3>
                    <p>No account or dashboard. Just send fare-drop alerts to this address.</p>
                    <label htmlFor="alert-email">Email address</label>
                    <input
                      id="alert-email"
                      type="email"
                      value={email}
                      onChange={(e) => setEmail(e.target.value)}
                      placeholder="you@example.com"
                      required
                    />
                    <button className="btn btn-outline" type="submit" disabled={emailBusy}>
                      {emailBusy ? 'Saving…' : 'Email me price drops'}
                    </button>
                  </form>
                </div>
              )}
            </section>
          </div>
        )}

        {bookingOffer && (
          <div className={styles.modalBackdrop} role="presentation" onMouseDown={() => setBookingOffer(null)}>
            <section
              className={styles.modal}
              role="dialog"
              aria-modal="true"
              aria-labelledby="booking-title"
              onMouseDown={(e) => e.stopPropagation()}
            >
              <button className={styles.close} type="button" aria-label="Close" onClick={() => setBookingOffer(null)}>
                ×
              </button>
              <p className={styles.eyebrow}>Book this itinerary</p>
              <h2 id="booking-title">
                {bookingOffer.airline} {bookingOffer.flightNumber}
              </h2>
              <p className={styles.modalRoute}>
                FareWatch doesn’t sell tickets. These open the airline, an OTA, or Google Flights.
              </p>
              {bookingError && <p className={styles.modalError}>{bookingError}</p>}
              {bookingBusy && <p className={styles.empty}>Finding checkout links…</p>}
              <div className={styles.bookingList}>
                {bookingLinks.map((link) => (
                  <a
                    key={`${link.providerName}-${link.url}`}
                    className={styles.bookingItem}
                    href={link.url}
                    target="_blank"
                    rel="noreferrer"
                  >
                    <div>
                      <strong>{link.providerName}</strong>
                      <span>
                        {link.providerType}
                        {link.fareName ? ` · ${link.fareName}` : ''}
                      </span>
                    </div>
                    <div className={styles.bookingPrice}>
                      {link.price > 0 ? formatPrice(link.price, link.currency) : 'Open →'}
                    </div>
                  </a>
                ))}
              </div>
            </section>
          </div>
        )}
      </div>
    </div>
  );
}
