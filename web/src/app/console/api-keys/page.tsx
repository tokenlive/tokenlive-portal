"use client";

import { useState, useEffect, useCallback } from "react";
import { useRouter } from "next/navigation";
import { listAPIKeys, createAPIKey, updateAPIKeyStatus } from "@/lib/api";
import { buildCreateAPIKeyRequest, formatAPIKeyLimit } from "@/lib/api-key-form";
import { getConsoleAuthRedirect } from "@/lib/auth-flow";
import type { APIKeyResponse } from "@/types/api";

export default function APIKeysPage() {
  const router = useRouter();
  const [keys, setKeys] = useState<APIKeyResponse[]>([]);
  const [loading, setLoading] = useState(true);
  const [busyKeyID, setBusyKeyID] = useState<string | null>(null);
  const [showCreate, setShowCreate] = useState(false);
  const [newKeyName, setNewKeyName] = useState("");
  const [dailyLimitCNY, setDailyLimitCNY] = useState("");
  const [monthlyLimitCNY, setMonthlyLimitCNY] = useState("");
  const [expiresOn, setExpiresOn] = useState("");
  const [createdSecret, setCreatedSecret] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);

  const loadKeys = useCallback(async () => {
    try {
      const res = await listAPIKeys();
      setKeys(res.data);
    } catch (err) {
      const redirectTo = getConsoleAuthRedirect(err);
      if (redirectTo) {
        router.replace(redirectTo);
        return;
      }
      setError(err instanceof Error ? err.message : "Failed to load API keys");
    } finally {
      setLoading(false);
    }
  }, [router]);

  useEffect(() => {
    void Promise.resolve().then(loadKeys);
  }, [loadKeys]);

  const handleCreate = async (e: React.FormEvent) => {
    e.preventDefault();
    setError(null);
    try {
      const res = await createAPIKey(
        buildCreateAPIKeyRequest({
          name: newKeyName,
          dailyLimitCNY,
          monthlyLimitCNY,
          expiresOn,
        })
      );
      setCreatedSecret(res.secret);
      setNewKeyName("");
      setDailyLimitCNY("");
      setMonthlyLimitCNY("");
      setExpiresOn("");
      setShowCreate(false);
      loadKeys();
    } catch (err) {
      const redirectTo = getConsoleAuthRedirect(err);
      if (redirectTo) {
        router.replace(redirectTo);
        return;
      }
      setError(err instanceof Error ? err.message : "Failed to create key");
    }
  };

  const handleStatusChange = async (
    id: string,
    action: "enable" | "disable" | "revoke"
  ) => {
    if (action === "revoke" && !window.confirm("Revoke this API key? This cannot be undone.")) {
      return;
    }

    setBusyKeyID(id);
    setError(null);
    try {
      await updateAPIKeyStatus(id, action);
      loadKeys();
    } catch (err) {
      const redirectTo = getConsoleAuthRedirect(err);
      if (redirectTo) {
        router.replace(redirectTo);
        return;
      }
      setError(err instanceof Error ? err.message : `Failed to ${action} key`);
    } finally {
      setBusyKeyID(null);
    }
  };

  const handleCopySecret = () => {
    if (createdSecret) {
      navigator.clipboard.writeText(createdSecret);
    }
  };

  return (
    <div className="p-6 lg:p-8">
      <div className="mb-8 flex items-center justify-between">
        <div>
          <div className="flex items-center gap-2 mb-1">
            <span className="tl-node tl-node--active" />
            <h1 className="font-heading text-xl font-semibold text-foreground">
              API Keys
            </h1>
          </div>
          <p className="text-sm text-muted-foreground">
            Manage keys for accessing the unified API gateway.
          </p>
        </div>
        <button
          onClick={() => setShowCreate(true)}
          className="rounded-md bg-primary px-4 py-2 text-sm font-medium text-primary-foreground transition-colors hover:bg-primary/90 tl-focus-ring"
        >
          Create key
        </button>
      </div>

      {/* Created secret banner */}
      {createdSecret && (
        <div className="mb-6 rounded-lg border border-primary/30 bg-primary/5 p-4">
          <p className="text-sm font-medium text-primary mb-2">
            Key created — copy it now, it won&apos;t be shown again:
          </p>
          <div className="flex items-center gap-2">
            <code className="flex-1 rounded-md bg-background px-3 py-2 font-mono text-sm text-foreground select-all">
              {createdSecret}
            </code>
            <button
              onClick={handleCopySecret}
              className="rounded-md border border-border px-3 py-2 text-sm text-muted-foreground transition-colors hover:text-foreground hover:bg-secondary"
            >
              Copy
            </button>
            <button
              onClick={() => setCreatedSecret(null)}
              className="rounded-md border border-border px-3 py-2 text-sm text-muted-foreground transition-colors hover:text-foreground hover:bg-secondary"
            >
              Dismiss
            </button>
          </div>
        </div>
      )}

      {/* Create form */}
      {showCreate && (
        <form
          onSubmit={handleCreate}
          className="mb-6 rounded-lg border border-border bg-card p-4"
        >
          <div className="grid gap-4 lg:grid-cols-[1fr_180px_180px_180px]">
            <Field label="Key name" htmlFor="key-name">
              <input
                id="key-name"
                type="text"
                required
                value={newKeyName}
                onChange={(e) => setNewKeyName(e.target.value)}
                placeholder="e.g. production"
                className="h-10 w-full rounded-md border border-input bg-background px-3 text-sm text-foreground placeholder:text-muted-foreground focus:outline-none focus:ring-2 focus:ring-primary/40"
                autoFocus
              />
            </Field>
            <Field label="Daily limit" htmlFor="daily-limit">
              <MoneyInput
                id="daily-limit"
                value={dailyLimitCNY}
                onChange={setDailyLimitCNY}
                placeholder="No limit"
              />
            </Field>
            <Field label="Monthly limit" htmlFor="monthly-limit">
              <MoneyInput
                id="monthly-limit"
                value={monthlyLimitCNY}
                onChange={setMonthlyLimitCNY}
                placeholder="No limit"
              />
            </Field>
            <Field label="Expires on" htmlFor="expires-on">
              <input
                id="expires-on"
                type="date"
                value={expiresOn}
                onChange={(e) => setExpiresOn(e.target.value)}
                className="h-10 w-full rounded-md border border-input bg-background px-3 text-sm text-foreground placeholder:text-muted-foreground focus:outline-none focus:ring-2 focus:ring-primary/40"
              />
            </Field>
          </div>
          <div className="mt-4 flex justify-end gap-3">
            <button
              type="button"
              onClick={() => setShowCreate(false)}
              className="rounded-md border border-border px-4 py-2 text-sm text-muted-foreground transition-colors hover:bg-secondary hover:text-foreground"
            >
              Cancel
            </button>
            <button
              type="submit"
              disabled={!newKeyName.trim()}
              className="rounded-md bg-primary px-4 py-2 text-sm font-medium text-primary-foreground transition-colors hover:bg-primary/90 disabled:opacity-50"
            >
              Create key
            </button>
          </div>
          {error && (
            <p className="mt-2 text-sm text-destructive">{error}</p>
          )}
        </form>
      )}

      {error && !showCreate && (
        <div className="mb-6 rounded-lg border border-destructive/30 bg-destructive/10 p-4 text-sm text-destructive">
          {error}
        </div>
      )}

      {/* Keys table */}
      {loading ? (
        <div className="py-12 text-center text-sm text-muted-foreground">
          Loading keys...
        </div>
      ) : keys.length === 0 ? (
        <div className="rounded-lg border border-dashed border-border py-12 text-center">
          <p className="text-sm text-muted-foreground">
            No API keys yet. Create one to get started.
          </p>
        </div>
      ) : (
        <div className="rounded-lg border border-border bg-card overflow-hidden">
          <div className="overflow-x-auto">
            <table className="w-full text-sm">
              <thead>
                <tr className="border-b border-border bg-muted/30">
                  <th className="px-4 py-3 text-left font-medium text-muted-foreground">
                    Name
                  </th>
                  <th className="px-4 py-3 text-left font-medium text-muted-foreground">
                    Key
                  </th>
                  <th className="px-4 py-3 text-left font-medium text-muted-foreground">
                    Status
                  </th>
                  <th className="px-4 py-3 text-left font-medium text-muted-foreground">
                    Spend
                  </th>
                  <th className="px-4 py-3 text-left font-medium text-muted-foreground">
                    Limits
                  </th>
                  <th className="px-4 py-3 text-right font-medium text-muted-foreground">
                    Expires
                  </th>
                  <th className="px-4 py-3 text-right font-medium text-muted-foreground">
                    Actions
                  </th>
                </tr>
              </thead>
              <tbody>
                {keys.map((key) => (
                  <tr
                    key={key.id}
                    className="border-b border-border last:border-b-0 hover:bg-muted/20"
                  >
                    <td className="px-4 py-3 font-medium text-foreground">
                      {key.name}
                    </td>
                    <td className="px-4 py-3 font-mono text-muted-foreground">
                      {key.key_prefix}
                      {key.secret_last4}
                    </td>
                    <td className="px-4 py-3">
                      <StatusBadge status={key.status} />
                    </td>
                    <td className="px-4 py-3 text-muted-foreground">
                      {key.total_spend_cny}
                    </td>
                    <td className="px-4 py-3 text-muted-foreground">
                      <div className="space-y-0.5 text-xs">
                        <p>Daily {formatAPIKeyLimit(key.daily_limit_micro_cny)}</p>
                        <p>Monthly {formatAPIKeyLimit(key.monthly_limit_micro_cny)}</p>
                      </div>
                    </td>
                    <td className="px-4 py-3 text-right text-muted-foreground">
                      {key.expires_at
                        ? new Date(key.expires_at).toLocaleDateString()
                        : "Never"}
                    </td>
                    <td className="px-4 py-3 text-right">
                      <div className="flex items-center justify-end gap-1">
                        {key.status === "enabled" && (
                          <button
                            onClick={() =>
                              handleStatusChange(key.id, "disable")
                            }
                            disabled={busyKeyID === key.id}
                            className="rounded px-2 py-1 text-xs text-muted-foreground transition-colors hover:bg-secondary hover:text-foreground"
                          >
                            Disable
                          </button>
                        )}
                        {key.status === "disabled" && (
                          <button
                            onClick={() =>
                              handleStatusChange(key.id, "enable")
                            }
                            disabled={busyKeyID === key.id}
                            className="rounded px-2 py-1 text-xs text-muted-foreground transition-colors hover:bg-secondary hover:text-foreground"
                          >
                            Enable
                          </button>
                        )}
                        {key.status !== "revoked" && (
                          <button
                            onClick={() =>
                              handleStatusChange(key.id, "revoke")
                            }
                            disabled={busyKeyID === key.id}
                            className="rounded px-2 py-1 text-xs text-destructive transition-colors hover:bg-destructive/10"
                          >
                            Revoke
                          </button>
                        )}
                      </div>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </div>
      )}
    </div>
  );
}

function Field({
  label,
  htmlFor,
  children,
}: {
  label: string;
  htmlFor: string;
  children: React.ReactNode;
}) {
  return (
    <label htmlFor={htmlFor} className="block">
      <span className="mb-2 block text-sm font-medium text-foreground">
        {label}
      </span>
      {children}
    </label>
  );
}

function MoneyInput({
  id,
  value,
  onChange,
  placeholder,
}: {
  id: string;
  value: string;
  onChange: (value: string) => void;
  placeholder: string;
}) {
  return (
    <div className="relative">
      <span className="pointer-events-none absolute left-3 top-1/2 -translate-y-1/2 text-sm text-muted-foreground">
        ¥
      </span>
      <input
        id={id}
        type="number"
        min="0"
        step="0.01"
        value={value}
        onChange={(e) => onChange(e.target.value)}
        placeholder={placeholder}
        className="h-10 w-full rounded-md border border-input bg-background pl-7 pr-3 text-sm text-foreground placeholder:text-muted-foreground focus:outline-none focus:ring-2 focus:ring-primary/40"
      />
    </div>
  );
}

function StatusBadge({ status }: { status: string }) {
  const variants: Record<string, { dot: string; text: string; label: string }> =
    {
      enabled: {
        dot: "bg-green-500",
        text: "text-green-400",
        label: "Enabled",
      },
      disabled: {
        dot: "bg-muted-foreground",
        text: "text-muted-foreground",
        label: "Disabled",
      },
      revoked: {
        dot: "bg-destructive",
        text: "text-destructive",
        label: "Revoked",
      },
    };
  const v = variants[status] || variants.disabled;
  return (
    <span className={`inline-flex items-center gap-1.5 text-xs ${v.text}`}>
      <span className={`h-1.5 w-1.5 rounded-full ${v.dot}`} />
      {v.label}
    </span>
  );
}
