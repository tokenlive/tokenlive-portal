"use client";

import { useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import { fetchRequestLogs, fetchUsageSummary } from "@/lib/api";
import { getConsoleAuthRedirect } from "@/lib/auth-flow";
import {
  formatCostCNY,
  formatLatency,
  formatSuccessRate,
  formatTokens,
} from "@/lib/usage-display";
import type { RequestLogsResponse, UsageSummaryResponse } from "@/types/api";

export default function UsagePage() {
  const router = useRouter();
  const [summary, setSummary] = useState<UsageSummaryResponse | null>(null);
  const [logs, setLogs] = useState<RequestLogsResponse | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;

    async function loadUsage() {
      try {
        const [summaryData, logsData] = await Promise.all([
          fetchUsageSummary(),
          fetchRequestLogs(50),
        ]);
        if (!cancelled) {
          setSummary(summaryData);
          setLogs(logsData);
        }
      } catch (err) {
        if (cancelled) {
          return;
        }
        const redirectTo = getConsoleAuthRedirect(err);
        if (redirectTo) {
          router.replace(redirectTo);
          return;
        }
        setError(err instanceof Error ? err.message : "Failed to load usage");
      } finally {
        if (!cancelled) {
          setLoading(false);
        }
      }
    }

    loadUsage();

    return () => {
      cancelled = true;
    };
  }, [router]);

  const today = summary?.today;

  return (
    <div className="p-6 lg:p-8">
      <div className="mb-8">
        <div className="mb-1 flex items-center gap-2">
          <span className="tl-node tl-node--active" />
          <h1 className="font-heading text-xl font-semibold text-foreground">
            Usage
          </h1>
        </div>
        <p className="text-sm text-muted-foreground">
          Review today&apos;s gateway traffic and recent request outcomes.
        </p>
      </div>

      {error && (
        <div className="mb-6 rounded-lg border border-destructive/30 bg-destructive/10 p-4 text-sm text-destructive">
          {error}
        </div>
      )}

      {loading ? (
        <div className="rounded-lg border border-border bg-card p-5 text-sm text-muted-foreground">
          Loading usage...
        </div>
      ) : summary?.available === false ? (
        <div className="rounded-lg border border-border bg-card p-5 text-sm text-muted-foreground">
          Usage data is not available in this deployment.
        </div>
      ) : (
        <div className="space-y-6">
          <section className="grid gap-4 sm:grid-cols-2 xl:grid-cols-4">
            <MetricCard label="Requests" value={formatTokens(today?.request_count)} />
            <MetricCard
              label="Success rate"
              value={formatSuccessRate(today?.success_count, today?.request_count)}
            />
            <MetricCard label="Tokens" value={formatTokens(tokenTotal(today))} />
            <MetricCard label="Cost" value={formatCostCNY(today?.cost_cny)} />
          </section>

          <section className="grid gap-4 sm:grid-cols-3">
            <MetricCard label="Avg latency" value={formatLatency(today?.avg_latency_ms)} />
            <MetricCard label="Avg TTFT" value={formatLatency(today?.avg_ttft_ms)} />
            <MetricCard label="Errors" value={formatTokens(today?.error_count)} />
          </section>

          <section className="rounded-lg border border-border bg-card">
            <div className="border-b border-border px-5 py-4">
              <h2 className="font-heading text-sm font-medium uppercase tracking-wider text-muted-foreground">
                Models
              </h2>
            </div>
            {summary?.models.length ? (
              <div className="overflow-x-auto">
                <table className="w-full text-sm">
                  <thead>
                    <tr className="border-b border-border bg-muted/30">
                      <TableHead>Model</TableHead>
                      <TableHead align="right">Requests</TableHead>
                      <TableHead align="right">Success</TableHead>
                      <TableHead align="right">Tokens</TableHead>
                      <TableHead align="right">Cost</TableHead>
                    </tr>
                  </thead>
                  <tbody>
                    {summary.models.map((model) => (
                      <tr key={model.model} className="border-b border-border last:border-0">
                        <td className="px-4 py-3 font-medium text-foreground">
                          {model.model || "-"}
                        </td>
                        <TableCell align="right">
                          {formatTokens(model.request_count)}
                        </TableCell>
                        <TableCell align="right">
                          {formatSuccessRate(model.success_count, model.request_count)}
                        </TableCell>
                        <TableCell align="right">
                          {formatTokens(model.input_tokens + model.output_tokens)}
                        </TableCell>
                        <TableCell align="right">{formatCostCNY(model.cost_cny)}</TableCell>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            ) : (
              <EmptyState>No model usage today.</EmptyState>
            )}
          </section>

          <section className="rounded-lg border border-border bg-card">
            <div className="border-b border-border px-5 py-4">
              <h2 className="font-heading text-sm font-medium uppercase tracking-wider text-muted-foreground">
                Recent Requests
              </h2>
            </div>
            {logs?.logs.length ? (
              <div className="overflow-x-auto">
                <table className="w-full text-sm">
                  <thead>
                    <tr className="border-b border-border bg-muted/30">
                      <TableHead>Time</TableHead>
                      <TableHead>Model</TableHead>
                      <TableHead>API key</TableHead>
                      <TableHead align="right">Status</TableHead>
                      <TableHead align="right">Latency</TableHead>
                      <TableHead align="right">Cost</TableHead>
                    </tr>
                  </thead>
                  <tbody>
                    {logs.logs.map((log) => (
                      <tr key={log.request_id} className="border-b border-border last:border-0">
                        <td className="px-4 py-3 text-muted-foreground">
                          {formatDateTime(log.time)}
                        </td>
                        <td className="px-4 py-3 font-medium text-foreground">
                          {log.model || "-"}
                        </td>
                        <td className="px-4 py-3 text-muted-foreground">
                          {log.api_key_display || log.api_key_id || "-"}
                        </td>
                        <TableCell align="right">{log.status_code}</TableCell>
                        <TableCell align="right">{formatLatency(log.latency_ms)}</TableCell>
                        <TableCell align="right">{formatCostCNY(log.cost_cny)}</TableCell>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            ) : (
              <EmptyState>No recent requests.</EmptyState>
            )}
          </section>
        </div>
      )}
    </div>
  );
}

function tokenTotal(today: UsageSummaryResponse["today"] | undefined) {
  if (!today) {
    return 0;
  }
  return today.input_tokens + today.output_tokens;
}

function formatDateTime(value: string) {
  if (!value) {
    return "-";
  }
  return new Date(value).toLocaleString();
}

function MetricCard({ label, value }: { label: string; value: string }) {
  return (
    <div className="rounded-lg border border-border bg-card p-4">
      <p className="text-xs font-medium uppercase tracking-wider text-muted-foreground">
        {label}
      </p>
      <p className="mt-2 font-heading text-2xl font-semibold text-foreground">
        {value}
      </p>
    </div>
  );
}

function TableHead({
  children,
  align = "left",
}: {
  children: React.ReactNode;
  align?: "left" | "right";
}) {
  return (
    <th
      className={`px-4 py-3 font-medium text-muted-foreground ${
        align === "right" ? "text-right" : "text-left"
      }`}
    >
      {children}
    </th>
  );
}

function TableCell({
  children,
  align = "left",
}: {
  children: React.ReactNode;
  align?: "left" | "right";
}) {
  return (
    <td
      className={`px-4 py-3 text-muted-foreground ${
        align === "right" ? "text-right" : "text-left"
      }`}
    >
      {children}
    </td>
  );
}

function EmptyState({ children }: { children: React.ReactNode }) {
  return (
    <div className="px-5 py-12 text-center text-sm text-muted-foreground">
      {children}
    </div>
  );
}
