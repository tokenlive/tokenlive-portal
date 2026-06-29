"use client";

import { useState, useEffect, useCallback } from "react";
import Image from "next/image";
import { getMe, listOAuthAccounts, logout } from "@/lib/api";
import { getOAuthProviderRows } from "@/lib/oauth-providers";
import type { CurrentUser, AccountIdentityDTO } from "@/types/api";
import { useRouter } from "next/navigation";

export default function SettingsPage() {
  const router = useRouter();
  const [user, setUser] = useState<CurrentUser | null>(null);
  const [accounts, setAccounts] = useState<AccountIdentityDTO[]>([]);
  const [loading, setLoading] = useState(true);
  const providerRows = getOAuthProviderRows(accounts);

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
    void Promise.resolve().then(load);
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
              <Image
                src={user.avatar_url}
                alt={user.display_name}
                width={48}
                height={48}
                className="h-12 w-12 rounded-full object-cover"
                unoptimized
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
          {providerRows.map((row) =>
            row.connected && row.account ? (
              <div
                key={row.id}
                className="flex items-center justify-between p-4"
              >
                <div className="flex items-center gap-3">
                  <ProviderIcon provider={row.id} />
                  <div>
                    <p className="text-sm font-medium text-foreground capitalize">
                      {row.label}
                    </p>
                    <p className="text-xs text-muted-foreground">
                      {row.account.display_name} · {row.account.email}
                    </p>
                  </div>
                </div>
                <span className="inline-flex items-center gap-1.5 rounded-md bg-primary/10 px-2 py-1 text-xs font-medium text-primary">
                  <span className="tl-node tl-node--sm" />
                  Connected
                </span>
              </div>
            ) : (
              <div key={row.id} className="flex items-center justify-between p-4">
                <div className="flex items-center gap-3">
                  <ProviderIcon provider={row.id} />
                  <div>
                    <p className="text-sm font-medium text-foreground">
                      {row.label}
                    </p>
                    <p className="text-xs text-muted-foreground">
                      {row.disabled ? row.unavailableLabel : "Not connected"}
                    </p>
                  </div>
                </div>
                {row.enabled && row.bindHref ? (
                  <a
                    href={row.bindHref}
                    className="rounded-md border border-border px-3 py-1.5 text-sm text-foreground transition-colors hover:bg-secondary tl-focus-ring"
                  >
                    Connect
                  </a>
                ) : (
                  <button
                    type="button"
                    disabled
                    className="cursor-not-allowed rounded-md border border-border px-3 py-1.5 text-sm text-muted-foreground opacity-70"
                  >
                    Unavailable
                  </button>
                )}
              </div>
            )
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
  if (provider === "github") {
    return (
      <svg
        className="h-5 w-5 text-foreground"
        viewBox="0 0 24 24"
        fill="currentColor"
        aria-hidden="true"
      >
        <path d="M12 2C6.48 2 2 6.59 2 12.25c0 4.53 2.87 8.37 6.84 9.73.5.09.68-.22.68-.49 0-.24-.01-1.05-.01-1.9-2.51.47-3.16-.63-3.36-1.21-.11-.3-.6-1.21-1.03-1.46-.35-.19-.85-.66-.01-.67.79-.01 1.35.74 1.54 1.05.9 1.55 2.34 1.11 2.91.85.09-.67.35-1.11.64-1.37-2.22-.26-4.55-1.14-4.55-5.05 0-1.11.39-2.03 1.03-2.75-.1-.26-.45-1.3.1-2.71 0 0 .84-.27 2.75 1.05A9.3 9.3 0 0 1 12 6.98c.85 0 1.71.12 2.51.34 1.91-1.33 2.75-1.05 2.75-1.05.55 1.41.2 2.45.1 2.71.64.72 1.03 1.63 1.03 2.75 0 3.92-2.34 4.79-4.57 5.05.36.32.68.93.68 1.89 0 1.37-.01 2.47-.01 2.81 0 .27.18.59.69.49A10.08 10.08 0 0 0 22 12.25C22 6.59 17.52 2 12 2Z" />
      </svg>
    );
  }

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
