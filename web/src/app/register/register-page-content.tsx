"use client";

import { useState } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { Header } from "@/components/layout/header";
import { startEmailLogin } from "@/lib/api";

export default function RegisterPageContent() {
  const router = useRouter();
  const [email, setEmail] = useState("");
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError(null);
    setLoading(true);
    try {
      await startEmailLogin(email);
      // Redirect to login/verify page
      router.push(`/login?email=${encodeURIComponent(email)}&start=true`);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to send code");
    } finally {
      setLoading(false);
    }
  };

  return (
    <>
      <Header />
      <main className="flex flex-1 items-center justify-center px-4 py-12">
        <div className="relative w-full max-w-sm">
          {/* Ambient glow */}
          <div className="pointer-events-none absolute -top-24 left-1/2 h-48 w-72 -translate-x-1/2 rounded-full bg-primary/8 blur-2xl" />

          <div className="relative rounded-xl border border-border bg-card p-6 shadow-lg">
            {/* Title */}
            <div className="mb-6 text-center">
              <div className="mx-auto mb-3 flex items-center justify-center gap-2">
                <span className="tl-node tl-node--lg tl-node--active" />
                <span className="font-heading text-lg font-semibold">
                  TokenLive
                </span>
              </div>
              <p className="text-sm text-muted-foreground">
                Create your account
              </p>
            </div>

            {/* Registration form */}
            <form onSubmit={handleSubmit} className="space-y-4">
              <div>
                <label
                  htmlFor="email"
                  className="block text-sm font-medium text-foreground mb-1.5"
                >
                  Email address
                </label>
                <input
                  id="email"
                  type="email"
                  required
                  value={email}
                  onChange={(e) => setEmail(e.target.value)}
                  placeholder="you@example.com"
                  className="h-10 w-full rounded-md border border-input bg-background px-3 text-sm text-foreground placeholder:text-muted-foreground focus:outline-none focus:ring-2 focus:ring-primary/40"
                  autoFocus
                />
              </div>

              {error && (
                <p className="rounded-md bg-destructive/10 px-3 py-2 text-sm text-destructive">
                  {error}
                </p>
              )}

              <button
                type="submit"
                disabled={loading || !email}
                className="h-10 w-full rounded-md bg-primary text-sm font-medium text-primary-foreground transition-colors hover:bg-primary/90 disabled:opacity-50 tl-focus-ring"
              >
                {loading ? "Sending code..." : "Continue with email"}
              </button>
            </form>

            {/* Login link */}
            <p className="mt-4 text-center text-sm text-muted-foreground">
              Already have an account?{" "}
              <Link
                href="/login"
                className="text-foreground hover:text-primary transition-colors"
              >
                Sign in
              </Link>
            </p>
          </div>
        </div>
      </main>
    </>
  );
}
