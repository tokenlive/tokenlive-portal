"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import { Header } from "@/components/layout/header";
import { acceptTerms } from "@/lib/api";

export default function AcceptTermsPage() {
  const router = useRouter();
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const handleAccept = async () => {
    setError(null);
    setLoading(true);
    try {
      await acceptTerms();
      router.push("/console/dashboard");
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to accept terms");
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
              <h1 className="text-lg font-semibold text-foreground mb-2">
                Terms of Service
              </h1>
              <p className="text-sm text-muted-foreground">
                Please accept our terms to continue
              </p>
            </div>

            {/* Terms content */}
            <div className="mb-6 max-h-64 overflow-y-auto rounded-md border border-border bg-muted/30 p-4 text-sm text-muted-foreground">
              <p className="mb-3">
                By using TokenLive, you agree to these terms. Please read them carefully.
              </p>
              <p className="mb-3">
                <strong>API Usage</strong>: You are responsible for all API usage under your account, including any costs incurred.
              </p>
              <p className="mb-3">
                <strong>Security</strong>: You must keep your API keys secure and not share them publicly.
              </p>
              <p className="mb-3">
                <strong>Acceptable Use</strong>: You agree to use the service in compliance with all applicable laws and regulations.
              </p>
              <p>
                <strong>Liability</strong>: We provide the service "as is" without any warranties.
              </p>
            </div>

            {error && (
              <p className="mb-4 rounded-md bg-destructive/10 px-3 py-2 text-sm text-destructive">
                {error}
              </p>
            )}

            <button
              onClick={handleAccept}
              disabled={loading}
              className="h-10 w-full rounded-md bg-primary text-sm font-medium text-primary-foreground transition-colors hover:bg-primary/90 disabled:opacity-50 tl-focus-ring"
            >
              {loading ? "Accepting..." : "Accept Terms & Continue"}
            </button>
          </div>
        </div>
      </main>
    </>
  );
}
