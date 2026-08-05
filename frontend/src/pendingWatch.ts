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

export function loadPendingWatch(): PendingWatch | null {
  const raw = sessionStorage.getItem(PENDING_KEY);
  if (!raw) return null;
  try {
    return JSON.parse(raw) as PendingWatch;
  } catch {
    return null;
  }
}

export function clearPendingWatch() {
  sessionStorage.removeItem(PENDING_KEY);
}
