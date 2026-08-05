const API_URL = import.meta.env.VITE_API_URL ?? 'http://localhost:8080/graphql';
const TOKEN_KEY = 'farewatch_token';

type GraphQLResponse<T> = {
  data?: T;
  errors?: { message: string }[];
};

export function getToken() {
  return localStorage.getItem(TOKEN_KEY);
}

export function setToken(token: string | null) {
  if (token) localStorage.setItem(TOKEN_KEY, token);
  else localStorage.removeItem(TOKEN_KEY);
}

async function gql<T>(query: string, variables?: Record<string, unknown>): Promise<T> {
  const headers: Record<string, string> = { 'Content-Type': 'application/json' };
  const token = getToken();
  if (token) headers.Authorization = `Bearer ${token}`;

  const res = await fetch(API_URL, {
    method: 'POST',
    headers,
    body: JSON.stringify({ query, variables }),
  });
  if (!res.ok) throw new Error(`API ${res.status}`);
  const json = (await res.json()) as GraphQLResponse<T>;
  if (json.errors?.length) throw new Error(json.errors.map((e) => e.message).join(', '));
  if (!json.data) throw new Error('Empty GraphQL response');
  return json.data;
}

export type User = { id: string; email: string; name: string; createdAt: string };

export type Airport = { code: string; city: string; country: string };

export type Route = {
  id: string;
  origin: string;
  destination: string;
  departDate: string;
  returnDate?: string | null;
  cabin: string;
  active: boolean;
  createdAt: string;
};

export type Fare = {
  id: string;
  routeId: string;
  airline: string;
  airlineCode: string;
  flightNumber: string;
  origin: string;
  originCity: string;
  destination: string;
  destinationCity: string;
  departAt: string;
  arriveAt: string;
  durationMinutes: number;
  stops: number;
  cabin: string;
  aircraft: string;
  price: number;
  currency: string;
  deepLink: string;
  source: string;
  cached: boolean;
  observedAt: string;
};

export type FlightSegment = {
  airlineCode: string;
  airline: string;
  flightNumber: string;
  origin: string;
  originCity: string;
  destination: string;
  destinationCity: string;
  departAt: string;
  arriveAt: string;
  durationMinutes: number;
  aircraft: string;
};

export type FlightOffer = {
  offerId: string;
  airline: string;
  airlineCode: string;
  flightNumber: string;
  origin: string;
  originCity: string;
  destination: string;
  destinationCity: string;
  departAt: string;
  arriveAt: string;
  durationMinutes: number;
  stops: number;
  cabin: string;
  aircraft: string;
  price: number;
  currency: string;
  deepLink: string;
  source: string;
  layoverAirports: string[];
  segments: FlightSegment[];
};

export type Watch = {
  id: string;
  userId?: string;
  email: string;
  routeId: string;
  airlineCode?: string;
  flightNumber?: string;
  targetPrice?: number | null;
  notifyOnDrop: boolean;
  dropPercent: number;
  active: boolean;
  createdAt: string;
  change24h?: number | null;
  route?: Route | null;
  latestFare?: Fare | null;
};

export type Alert = {
  id: string;
  watchId: string;
  fareId: string;
  oldPrice: number;
  newPrice: number;
  airline: string;
  sentAt: string;
  deliveredInMs: number;
};

export type ScanStats = {
  routesScanned: number;
  faresFound: number;
  cacheHits: number;
  cacheMisses: number;
  cacheHitRate: number;
  alertsSent: number;
  durationMs: number;
  airlinesQueried: number;
};

export function register(input: { email: string; password: string; name: string }) {
  return gql<{ register: { token: string; user: User } }>(
    `mutation($email: String!, $password: String!, $name: String!) {
      register(email: $email, password: $password, name: $name) {
        token
        user { id email name createdAt }
      }
    }`,
    input,
  ).then((d) => {
    setToken(d.register.token);
    return d.register.user;
  });
}

export function login(input: { email: string; password: string }) {
  return gql<{ login: { token: string; user: User } }>(
    `mutation($email: String!, $password: String!) {
      login(email: $email, password: $password) {
        token
        user { id email name createdAt }
      }
    }`,
    input,
  ).then((d) => {
    setToken(d.login.token);
    return d.login.user;
  });
}

export function loginWithFirebase(idToken: string) {
  return gql<{ loginWithFirebase: { token: string; user: User } }>(
    `mutation($idToken: String!) {
      loginWithFirebase(idToken: $idToken) {
        token
        user { id email name createdAt }
      }
    }`,
    { idToken },
  ).then((d) => {
    setToken(d.loginWithFirebase.token);
    return d.loginWithFirebase.user;
  });
}

export function fetchMe() {
  return gql<{ me: User }>(`query { me { id email name createdAt } }`).then((d) => d.me);
}

export function fetchAirports() {
  return gql<{ airports: Airport[] }>(`query { airports { code city country } }`).then((d) => d.airports);
}

export function fetchMyWatches() {
  return gql<{ myWatches: Watch[] }>(`
    query {
      myWatches {
        id email routeId airlineCode flightNumber targetPrice notifyOnDrop dropPercent active createdAt change24h
        route { id origin destination departDate returnDate cabin }
        latestFare {
          id airline airlineCode flightNumber origin originCity destination destinationCity
          departAt arriveAt durationMinutes stops cabin aircraft price currency deepLink source cached observedAt
        }
      }
    }
  `).then((d) => d.myWatches);
}

export function searchFares(input: {
  origin: string;
  destination: string;
  departDate: string;
  returnDate?: string;
  cabin?: string;
}) {
  return gql<{ searchFares: FlightOffer[] }>(
    `query($origin: String!, $destination: String!, $departDate: String!, $returnDate: String, $cabin: String) {
      searchFares(origin: $origin, destination: $destination, departDate: $departDate, returnDate: $returnDate, cabin: $cabin) {
        offerId airline airlineCode flightNumber origin originCity destination destinationCity
        departAt arriveAt durationMinutes stops cabin aircraft price currency deepLink source
        layoverAirports
        segments {
          airlineCode airline flightNumber origin originCity destination destinationCity
          departAt arriveAt durationMinutes aircraft
        }
      }
    }`,
    input,
  ).then((d) => d.searchFares);
}

export function createWatch(input: {
  origin: string;
  destination: string;
  departDate: string;
  returnDate?: string;
  cabin?: string;
  airlineCode?: string;
  flightNumber?: string;
  targetPrice?: number;
  dropPercent?: number;
}) {
  return gql<{ createWatch: Watch }>(
    `mutation($origin: String!, $destination: String!, $departDate: String!, $returnDate: String, $cabin: String, $airlineCode: String, $flightNumber: String, $targetPrice: Float, $dropPercent: Float) {
      createWatch(origin: $origin, destination: $destination, departDate: $departDate, returnDate: $returnDate, cabin: $cabin, airlineCode: $airlineCode, flightNumber: $flightNumber, targetPrice: $targetPrice, dropPercent: $dropPercent) {
        id email routeId airlineCode flightNumber targetPrice active
        route { id origin destination departDate cabin }
      }
    }`,
    input,
  ).then((d) => d.createWatch);
}

export function createEmailWatch(input: {
  email: string;
  origin: string;
  destination: string;
  departDate: string;
  returnDate?: string;
  cabin?: string;
  airlineCode?: string;
  flightNumber?: string;
  targetPrice?: number;
}) {
  return gql<{ createEmailWatch: Watch }>(
    `mutation($email: String!, $origin: String!, $destination: String!, $departDate: String!, $returnDate: String, $cabin: String, $airlineCode: String, $flightNumber: String, $targetPrice: Float) {
      createEmailWatch(email: $email, origin: $origin, destination: $destination, departDate: $departDate, returnDate: $returnDate, cabin: $cabin, airlineCode: $airlineCode, flightNumber: $flightNumber, targetPrice: $targetPrice) {
        id email routeId airlineCode flightNumber targetPrice active
      }
    }`,
    input,
  ).then((d) => d.createEmailWatch);
}

export type BookingLink = {
  providerName: string;
  providerType: string;
  fareName: string;
  price: number;
  currency: string;
  url: string;
};

export function fetchBookingLinks(input: {
  offerId: string;
  origin?: string;
  destination?: string;
  departDate?: string;
  returnDate?: string;
}) {
  return gql<{ bookingLinks: BookingLink[] }>(
    `query($offerId: String!, $origin: String, $destination: String, $departDate: String, $returnDate: String) {
      bookingLinks(offerId: $offerId, origin: $origin, destination: $destination, departDate: $departDate, returnDate: $returnDate) {
        providerName providerType fareName price currency url
      }
    }`,
    input,
  ).then((d) => d.bookingLinks);
}

export function fetchFares(routeId: string, limit = 40) {
  return gql<{ fares: Fare[] }>(
    `query($routeId: ID!, $limit: Int) {
      fares(routeId: $routeId, limit: $limit) {
        id airline airlineCode flightNumber origin originCity destination destinationCity
        departAt arriveAt durationMinutes stops cabin aircraft price currency deepLink source cached observedAt
      }
    }`,
    { routeId, limit },
  ).then((d) => d.fares);
}

export function fetchMyAlerts(limit = 12) {
  return gql<{ myAlerts: Alert[] }>(
    `query($limit: Int) {
      myAlerts(limit: $limit) {
        id watchId fareId oldPrice newPrice airline sentAt deliveredInMs
      }
    }`,
    { limit },
  ).then((d) => d.myAlerts);
}

export function updateWatch(input: { id: string; notifyOnDrop?: boolean; targetPrice?: number }) {
  return gql<{ updateWatch: Watch }>(
    `mutation($id: ID!, $notifyOnDrop: Boolean, $targetPrice: Float) {
      updateWatch(id: $id, notifyOnDrop: $notifyOnDrop, targetPrice: $targetPrice) {
        id email routeId airlineCode flightNumber targetPrice notifyOnDrop dropPercent active createdAt
      }
    }`,
    input,
  ).then((d) => d.updateWatch);
}

export function removeWatch(id: string) {
  return gql<{ removeWatch: boolean }>(
    `mutation($id: ID!) { removeWatch(id: $id) }`,
    { id },
  ).then((d) => d.removeWatch);
}

export function runScan() {
  return gql<{ runScan: ScanStats }>(`
    mutation {
      runScan {
        routesScanned faresFound cacheHits cacheMisses cacheHitRate
        alertsSent durationMs airlinesQueried
      }
    }
  `).then((d) => d.runScan);
}

export function logout() {
  setToken(null);
}
