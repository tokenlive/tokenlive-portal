"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import { usePathname, useRouter } from "next/navigation";
import { getMe, logout } from "@/lib/api";
import { getConsoleAuthRedirect } from "@/lib/auth-flow";

const NAV_ITEMS = [
  { href: "/console/dashboard", label: "Overview", icon: "◉" },
  { href: "/console/api-keys", label: "API Keys", icon: "⚷" },
  { href: "/console/settings", label: "Settings", icon: "⚙" },
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
                <span className="text-base leading-none">{item.icon}</span>
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
            <span className="text-sm">⏻</span>
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
