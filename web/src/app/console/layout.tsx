"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import { usePathname, useRouter } from "next/navigation";
import {
  Activity,
  CircleGauge,
  CreditCard,
  KeyRound,
  Power,
  Settings,
} from "lucide-react";
import { getMe, logout } from "@/lib/api";
import { getConsoleAuthRedirect } from "@/lib/auth-flow";

const NAV_ITEMS = [
  { href: "/console/dashboard", label: "Overview", icon: CircleGauge },
  { href: "/console/api-keys", label: "API Keys", icon: KeyRound },
  { href: "/console/usage", label: "Usage", icon: Activity },
  { href: "/console/billing", label: "Billing", icon: CreditCard },
  { href: "/console/settings", label: "Settings", icon: Settings },
];

export default function ConsoleLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  const pathname = usePathname();
  const router = useRouter();
  const [ready, setReady] = useState(false);

  useEffect(() => {
    let cancelled = false;

    async function checkSession() {
      try {
        const user = await getMe();
        if (cancelled) {
          return;
        }
        if (!user.terms_accepted) {
          router.replace("/accept-terms");
          return;
        }
        setReady(true);
      } catch (err) {
        if (cancelled) {
          return;
        }
        const redirectTo = getConsoleAuthRedirect(err);
        if (redirectTo) {
          router.replace(redirectTo);
          return;
        }
        setReady(true);
      }
    }

    checkSession();

    return () => {
      cancelled = true;
    };
  }, [router]);

  const handleSignOut = async () => {
    try {
      await logout();
      router.push("/login");
    } catch {
      // Fallback: redirect even if logout fails
      router.push("/login");
    }
  };

  return (
    <div className="flex min-h-screen">
      {/* Sidebar */}
      <aside className="sticky top-0 flex h-screen w-56 flex-col border-r border-border bg-card">
        {/* Logo */}
        <Link
          href="/"
          className="flex h-14 items-center gap-2.5 border-b border-border px-4"
        >
          <span className="tl-node tl-node--lg tl-node--active" />
          <span className="font-heading text-base font-semibold text-foreground">
            TokenLive
          </span>
        </Link>

        {/* Nav */}
        <nav className="flex-1 space-y-1 px-3 py-4">
          {NAV_ITEMS.map((item) => {
            const active = pathname.startsWith(item.href);
            const Icon = item.icon;
            return (
              <Link
                key={item.href}
                href={item.href}
                className={`flex items-center gap-2.5 rounded-md px-3 py-2 text-sm font-medium transition-colors tl-focus-ring ${
                  active
                    ? "bg-secondary text-foreground"
                    : "text-muted-foreground hover:bg-secondary/60 hover:text-foreground"
                }`}
              >
                <Icon className="h-4 w-4" aria-hidden="true" />
                {item.label}
                {active && <span className="tl-node tl-node--sm ml-auto" />}
              </Link>
            );
          })}
        </nav>

        {/* Footer */}
        <div className="border-t border-border px-4 py-3">
          <button
            onClick={handleSignOut}
            className="flex w-full items-center gap-2 rounded-md px-2 py-1.5 text-sm text-muted-foreground transition-colors hover:text-foreground hover:bg-secondary/60"
          >
            <Power className="h-4 w-4" aria-hidden="true" />
            Sign out
          </button>
        </div>
      </aside>

      {/* Main content */}
      <main className="flex-1">
        {ready ? (
          children
        ) : (
          <div className="p-6 lg:p-8">
            <p className="text-sm text-muted-foreground">Checking session...</p>
          </div>
        )}
      </main>
    </div>
  );
}
