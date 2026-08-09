const PENDING_KEY = 'farewatch_pending_watch';

export type PendingWatch = {
  origin: string;
  destination: string;
  departDate: string;
  returnDate?: string;
  cabin: string;
  airlineCode: string;
  flightNumber: string;
  airline: string;
  price: number;
  currency: string;
  targetPrice: number;
  stops: number;
  layoverAirports: string[];
  departAt: string;
  arriveAt: string;
};

export function savePendingWatch(p: PendingWatch) {
  sessionStorage.setItem(PENDING_KEY, JSON.stringify(p));
}

function isPendingWatch(value: unknown): value is PendingWatch {
  if (!value || typeof value !== 'object') return false;
  const p = value as Record<string, unknown>;
  return (
    typeof p.origin === 'string' &&
    typeof p.destination === 'string' &&
    typeof p.departDate === 'string' &&
    typeof p.cabin === 'string' &&
    typeof p.airlineCode === 'string' &&
    typeof p.flightNumber === 'string' &&
    typeof p.airline === 'string' &&
    typeof p.price === 'number' &&
    typeof p.currency === 'string' &&
    typeof p.targetPrice === 'number'
  );
}

export function loadPendingWatch(): PendingWatch | null {
  const raw = sessionStorage.getItem(PENDING_KEY);
  if (!raw) return null;
  try {
    const parsed: unknown = JSON.parse(raw);
    // Data we wrote ourselves in a prior session, but the shape can drift
    // across deploys - validate before trusting it instead of a blind cast.
    if (!isPendingWatch(parsed)) {
      sessionStorage.removeItem(PENDING_KEY);
      return null;
    }
    return parsed;
  } catch {
    return null;
  }
}

export function clearPendingWatch() {
  sessionStorage.removeItem(PENDING_KEY);
}
