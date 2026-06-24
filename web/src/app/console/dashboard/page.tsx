import { fetchOverview } from "@/lib/api";
import type { Metadata } from "next";

export const metadata: Metadata = {
  title: "Dashboard",
};

export const dynamic = "force-dynamic";

export default async function DashboardPage() {
  let overview = null;
  try {
    overview = await fetchOverview();
  } catch {
    // API unreachable
  }

  const ws = overview?.workspace;
  const activation = overview?.activation;

  return (
    <div className="p-6 lg:p-8">
      {/* Header */}
      <div className="mb-8">
        <div className="flex items-center gap-2 mb-1">
          <span className="tl-node tl-node--active" />
          <h1 className="font-heading text-xl font-semibold text-foreground">
            Console
          </h1>
        </div>
        <p className="text-sm text-muted-foreground">
          Monitor your API usage, manage keys, and control spending.
        </p>
      </div>

      {/* Activation steps */}
      {activation && (
        <section className="mb-8">
          <h2 className="font-heading text-sm font-medium text-muted-foreground uppercase tracking-wider mb-4">
            Getting Started
          </h2>
          <div className="grid grid-cols-1 gap-3 sm:grid-cols-3">
            {activation.steps.map((step) => (
              <ActivationStepCard key={step.key} step={step} />
            ))}
          </div>
        </section>
      )}

      {/* Stats */}
      <section className="mb-8">
        <h2 className="font-heading text-sm font-medium text-muted-foreground uppercase tracking-wider mb-4">
          Workspace
        </h2>
        <div className="grid grid-cols-1 gap-4 sm:grid-cols-3">
          <StatCard
            label="Available Balance"
            value={ws?.balance.available_cny || "—"}
          />
          <StatCard
            label="Frozen Balance"
            value={ws?.balance.frozen_cny || "—"}
          />
          <StatCard
            label="Workspace"
            value={ws?.name || "—"}
            suffix={ws?.role ? `(${ws.role})` : undefined}
          />
        </div>
      </section>

      {/* Quick API snippet */}
      <section>
        <h2 className="font-heading text-sm font-medium text-muted-foreground uppercase tracking-wider mb-4">
          Quick Start
        </h2>
        <div className="rounded-lg border border-border bg-card p-5">
          <p className="mb-3 text-sm text-muted-foreground">
            Create an API key, then use it to call any model:
          </p>
          <pre className="overflow-x-auto rounded-md bg-background p-4 font-mono text-sm leading-relaxed text-foreground">
            <code>{`curl -X POST https://api.tokenlive.ai/v1/chat/completions \\
  -H "Authorization: Bearer tl_live_YOUR_API_KEY" \\
  -H "Content-Type: application/json" \\
  -d '{
    "model": "gpt-4o",
    "messages": [{"role": "user", "content": "Hello!"}]
  }'`}</code>
          </pre>
        </div>
      </section>
    </div>
  );
}

function ActivationStepCard({
  step,
}: {
  step: { key: string; label: string; status: string };
}) {
  const completed = step.status === "completed";
  return (
    <div
      className={`rounded-lg border p-4 transition-colors ${
        completed
          ? "border-primary/20 bg-primary/5"
          : "border-border bg-card"
      }`}
    >
      <div className="flex items-center gap-2 mb-1.5">
        <span
          className={`tl-node tl-node--sm ${
            completed ? "tl-node--active" : ""
          }`}
        />
        <h3 className="font-heading text-sm font-medium text-foreground">
          {step.label}
        </h3>
      </div>
    </div>
  );
}

function StatCard({
  label,
  value,
  suffix,
}: {
  label: string;
  value: string;
  suffix?: string;
}) {
  return (
    <div className="rounded-lg border border-border bg-card p-4">
      <p className="text-xs font-medium text-muted-foreground uppercase tracking-wider">
        {label}
      </p>
      <p className="mt-2 font-heading text-2xl font-bold text-foreground">
        {value}
        {suffix && (
          <span className="ml-1.5 text-sm font-normal text-muted-foreground">
            {suffix}
          </span>
        )}
      </p>
    </div>
  );
}
