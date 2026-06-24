"use client";

import { useState, useEffect, useCallback } from "react";
import { getMe, listOAuthAccounts, GOOGLE_BIND_URL, logout } from "@/lib/api";
import type { CurrentUser, AccountIdentityDTO } from "@/types/api";
import { useRouter } from "next/navigation";

export default function SettingsPage() {
  const router = useRouter();
  const [user, setUser] = useState<CurrentUser | null>(null);
  const [accounts, setAccounts] = useState<AccountIdentityDTO[]>([]);
  const [loading, setLoading] = useState(true);

  const load = useCallback(async () => {
    try {
      const [u, a] = await Promise.all([getMe(), listOAuthAccounts()]);
      setUser(u);
      setAccounts(a);
    } catch {
      // API unreachable
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    load();
  }, [load]);

  const handleSignOut = async () => {
    try {
      await logout();
      router.push("/login");
    } catch {
      // Fallback: redirect even if logout fails
      router.push("/login");
    }
  };

  if (loading) {
    return (
      <div className="p-6 lg:p-8">
        <p className="text-sm text-muted-foreground">Loading...</p>
      </div>
    );
  }

  return (
    <div className="p-6 lg:p-8 max-w-2xl">
      <div className="mb-8">
        <div className="flex items-center gap-2 mb-1">
          <span className="tl-node tl-node--active" />
          <h1 className="font-heading text-xl font-semibold text-foreground">
            Settings
          </h1>
        </div>
        <p className="text-sm text-muted-foreground">
          Manage your account and connected services.
        </p>
      </div>

      {/* Profile */}
      <section className="mb-8">
        <h2 className="font-heading text-sm font-medium text-muted-foreground uppercase tracking-wider mb-4">
          Profile
        </h2>
        <div className="rounded-lg border border-border bg-card p-5">
          <div className="flex items-center gap-4">
            {user?.avatar_url ? (
              <img
                src={user.avatar_url}
                alt={user.display_name}
                className="h-12 w-12 rounded-full object-cover"
              />
            ) : (
              <div className="flex h-12 w-12 items-center justify-center rounded-full bg-muted font-heading text-lg font-semibold text-muted-foreground">
                {user?.display_name?.charAt(0)?.toUpperCase() || "?"}
              </div>
            )}
            <div>
              <p className="font-heading font-medium text-foreground">
                {user?.display_name || "—"}
              </p>
              <p className="text-sm text-muted-foreground">
                {user?.primary_email || "—"}
              </p>
            </div>
          </div>
        </div>
      </section>

      {/* Connected Accounts */}
      <section className="mb-8">
        <h2 className="font-heading text-sm font-medium text-muted-foreground uppercase tracking-wider mb-4">
          Connected Accounts
        </h2>
        <div className="rounded-lg border border-border bg-card divide-y divide-border">
          {accounts.length > 0 &&
            accounts.map((acc) => (
              <div
                key={acc.provider}
                className="flex items-center justify-between p-4"
              >
                <div className="flex items-center gap-3">
                  <ProviderIcon provider={acc.provider} />
                  <div>
                    <p className="text-sm font-medium text-foreground capitalize">
                      {acc.provider}
                    </p>
                    <p className="text-xs text-muted-foreground">
                      {acc.display_name} · {acc.email}
                    </p>
                  </div>
                </div>
                <span className="inline-flex items-center gap-1.5 rounded-md bg-primary/10 px-2 py-1 text-xs font-medium text-primary">
                  <span className="tl-node tl-node--sm" />
                  Connected
                </span>
              </div>
            ))}

          {/* Always show Google bind option if not connected */}
          {!accounts.some((a) => a.provider === "google") && (
            <div className="flex items-center justify-between p-4">
              <div className="flex items-center gap-3">
                <ProviderIcon provider="google" />
                <div>
                  <p className="text-sm font-medium text-foreground">Google</p>
                  <p className="text-xs text-muted-foreground">
                    Not connected
                  </p>
                </div>
              </div>
              <a
                href={GOOGLE_BIND_URL}
                className="rounded-md border border-border px-3 py-1.5 text-sm text-foreground transition-colors hover:bg-secondary"
              >
                Connect
              </a>
            </div>
          )}
        </div>
      </section>

      {/* Danger zone */}
      <section>
        <h2 className="font-heading text-sm font-medium text-muted-foreground uppercase tracking-wider mb-4">
          Session
        </h2>
        <div className="rounded-lg border border-border bg-card p-5">
          <button
            onClick={handleSignOut}
            className="rounded-md border border-border px-4 py-2 text-sm font-medium text-foreground transition-colors hover:bg-destructive/10 hover:text-destructive hover:border-destructive/30"
          >
            Sign out
          </button>
        </div>
      </section>
    </div>
  );
}

function ProviderIcon({ provider }: { provider: string }) {
  if (provider === "google") {
    return (
      <svg className="h-5 w-5" viewBox="0 0 24 24">
        <path
          d="M22.56 12.25c0-.78-.07-1.53-.2-2.25H12v4.26h5.92a5.06 5.06 0 0 1-2.2 3.32v2.77h3.57c2.08-1.92 3.28-4.74 3.28-8.1z"
          fill="#4285F4"
        />
        <path
          d="M12 23c2.97 0 5.46-.98 7.28-2.66l-3.57-2.77c-.98.66-2.23 1.06-3.71 1.06-2.86 0-5.29-1.93-6.16-4.53H2.18v2.84C3.99 20.53 7.7 23 12 23z"
          fill="#34A853"
        />
        <path
          d="M5.84 14.09c-.22-.66-.35-1.36-.35-2.09s.13-1.43.35-2.09V7.07H2.18C1.43 8.55 1 10.22 1 12s.43 3.45 1.18 4.93l2.85-2.22.81-.62z"
          fill="#FBBC05"
        />
        <path
          d="M12 5.38c1.62 0 3.06.56 4.21 1.64l3.15-3.15C17.45 2.09 14.97 1 12 1 7.7 1 3.99 3.47 2.18 7.07l3.66 2.84c.87-2.6 3.3-4.53 6.16-4.53z"
          fill="#EA4335"
        />
      </svg>
    );
  }
  return (
    <div className="flex h-5 w-5 items-center justify-center rounded bg-muted text-xs font-mono text-muted-foreground">
      {provider.charAt(0).toUpperCase()}
    </div>
  );
}
