import { initializeApp, type FirebaseApp } from 'firebase/app';
import {
  getAuth,
  GoogleAuthProvider,
  onAuthStateChanged,
  signInWithPopup,
  signOut,
  type Auth,
  type User,
} from 'firebase/auth';

const config = {
  apiKey: import.meta.env.VITE_FIREBASE_API_KEY as string | undefined,
  authDomain: import.meta.env.VITE_FIREBASE_AUTH_DOMAIN as string | undefined,
  projectId: import.meta.env.VITE_FIREBASE_PROJECT_ID as string | undefined,
  appId: import.meta.env.VITE_FIREBASE_APP_ID as string | undefined,
};

export function firebaseEnabled() {
  return Boolean(config.apiKey && config.authDomain && config.projectId && config.appId);
}

let app: FirebaseApp | null = null;
let auth: Auth | null = null;
let ready: Promise<User | null> | null = null;

function getFirebaseAuth() {
  if (!firebaseEnabled()) return null;
  if (!app) {
    app = initializeApp({
      apiKey: config.apiKey,
      authDomain: config.authDomain,
      projectId: config.projectId,
      appId: config.appId,
    });
    auth = getAuth(app);
  }
  return auth;
}

/** Wait until Firebase restores any persisted Google session from a previous visit. */
export function waitForFirebaseUser(): Promise<User | null> {
  const a = getFirebaseAuth();
  if (!a) return Promise.resolve(null);
  if (!ready) {
    ready = new Promise((resolve) => {
      const unsub = onAuthStateChanged(a, (user) => {
        unsub();
        resolve(user);
      });
    });
  }
  return ready;
}

export async function getFirebaseIdToken(): Promise<string | null> {
  const user = await waitForFirebaseUser();
  if (!user) return null;
  return user.getIdToken(true);
}

export async function signInWithGoogle(): Promise<string> {
  const a = getFirebaseAuth();
  if (!a) throw new Error('Firebase is not configured');
  const provider = new GoogleAuthProvider();
  provider.addScope('email');
  provider.addScope('profile');
  provider.setCustomParameters({ prompt: 'select_account' });
  const cred = await signInWithPopup(a, provider);
  // Force a fresh token so the API verify step never gets a stale/cached JWT.
  return cred.user.getIdToken(true);
}

export async function signOutFirebase(): Promise<void> {
  const a = getFirebaseAuth();
  if (!a) return;
  await signOut(a);
  ready = Promise.resolve(null);
}
