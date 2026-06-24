"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";

export function Header() {
  const pathname = usePathname();
  const isConsole = pathname.startsWith("/console");

  return (
    <header className="sticky top-0 z-50 w-full border-b border-border/40 bg-background/80 backdrop-blur-xl">
      <div className="mx-auto flex h-14 max-w-6xl items-center justify-between px-4 sm:px-6">
        {/* Logo */}
        <Link href="/" className="flex items-center gap-2.5 group">
          <span className="tl-node tl-node--lg tl-node--active" />
          <span className="font-heading text-lg font-semibold tracking-tight text-foreground">
            TokenLive
          </span>
        </Link>

        {/* Nav */}
        <nav className="flex items-center gap-1">
          <NavLink href="/" active={pathname === "/"}>
            Models
          </NavLink>
          <NavLink href="/console/dashboard" active={isConsole}>
            Console
          </NavLink>
          <span className="mx-2 h-5 w-px bg-border" />
          <Link
            href="/login"
            className="ml-1 rounded-md bg-primary px-4 py-1.5 text-sm font-medium text-primary-foreground transition-colors hover:bg-primary/90 tl-focus-ring"
          >
            Sign in
          </Link>
        </nav>
      </div>
    </header>
  );
}

function NavLink({
  href,
  active,
  children,
}: {
  href: string;
  active: boolean;
  children: React.ReactNode;
}) {
  return (
    <Link
      href={href}
      className={`relative rounded-md px-3 py-1.5 text-sm font-medium transition-colors tl-focus-ring ${
        active
          ? "text-foreground"
          : "text-muted-foreground hover:text-foreground"
      }`}
    >
      {children}
      {active && (
        <span className="absolute inset-x-2 -bottom-[7px] h-px bg-primary" />
      )}
    </Link>
  );
}
