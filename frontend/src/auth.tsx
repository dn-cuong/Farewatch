import {
  createContext,
  useContext,
  useEffect,
  useState,
  type ReactNode,
} from 'react';
import {
  fetchMe,
  getToken,
  loginWithFirebase,
  logout as apiLogout,
  type User,
} from './api';
import {
  firebaseEnabled,
  getFirebaseIdToken,
  signOutFirebase,
} from './firebase';

type AuthCtx = {
  user: User | null;
  loading: boolean;
  setUser: (u: User | null) => void;
  logout: () => void;
  refresh: () => Promise<void>;
};

const Ctx = createContext<AuthCtx | null>(null);

export function AuthProvider({ children }: { children: ReactNode }) {
  const [user, setUser] = useState<User | null>(null);
  const [loading, setLoading] = useState(true);

  async function exchangeFirebaseSession(): Promise<User | null> {
    if (!firebaseEnabled()) return null;
    const idToken = await getFirebaseIdToken();
    if (!idToken) return null;
    return loginWithFirebase(idToken);
  }

  async function refresh() {
    if (getToken()) {
      try {
        const me = await fetchMe();
        setUser(me);
        return;
      } catch {
        apiLogout();
      }
    }

    try {
      const u = await exchangeFirebaseSession();
      setUser(u);
    } catch {
      apiLogout();
      await signOutFirebase().catch(() => undefined);
      setUser(null);
    }
  }

  useEffect(() => {
    refresh().finally(() => setLoading(false));
  }, []);

  return (
    <Ctx.Provider
      value={{
        user,
        loading,
        setUser,
        refresh,
        logout: () => {
          apiLogout();
          void signOutFirebase();
          setUser(null);
        },
      }}
    >
      {children}
    </Ctx.Provider>
  );
}

export function useAuth() {
  const ctx = useContext(Ctx);
  if (!ctx) throw new Error('useAuth outside provider');
  return ctx;
}
