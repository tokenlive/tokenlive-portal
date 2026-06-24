"use client";

import { useState, useEffect, useCallback } from "react";
import { listAPIKeys, createAPIKey, updateAPIKeyStatus } from "@/lib/api";
import type { APIKeyResponse } from "@/types/api";

export default function APIKeysPage() {
  const [keys, setKeys] = useState<APIKeyResponse[]>([]);
  const [loading, setLoading] = useState(true);
  const [showCreate, setShowCreate] = useState(false);
  const [newKeyName, setNewKeyName] = useState("");
  const [createdSecret, setCreatedSecret] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);

  const loadKeys = useCallback(async () => {
    try {
      const res = await listAPIKeys();
      setKeys(res.data);
    } catch {
      // API unreachable
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    loadKeys();
  }, [loadKeys]);

  const handleCreate = async (e: React.FormEvent) => {
    e.preventDefault();
    setError(null);
    try {
      const res = await createAPIKey({ name: newKeyName });
      setCreatedSecret(res.secret);
      setNewKeyName("");
      setShowCreate(false);
      loadKeys();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to create key");
    }
  };

  const handleStatusChange = async (
    id: string,
    action: "enable" | "disable" | "revoke"
  ) => {
    try {
      await updateAPIKeyStatus(id, action);
      loadKeys();
    } catch {
      // noop
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
          <label
            htmlFor="key-name"
            className="block text-sm font-medium text-foreground mb-2"
          >
            Key name
          </label>
          <div className="flex gap-3">
            <input
              id="key-name"
              type="text"
              required
              value={newKeyName}
              onChange={(e) => setNewKeyName(e.target.value)}
              placeholder="e.g. development, production"
              className="h-10 flex-1 rounded-md border border-input bg-background px-3 text-sm text-foreground placeholder:text-muted-foreground focus:outline-none focus:ring-2 focus:ring-primary/40"
              autoFocus
            />
            <button
              type="submit"
              disabled={!newKeyName}
              className="rounded-md bg-primary px-4 py-2 text-sm font-medium text-primary-foreground transition-colors hover:bg-primary/90 disabled:opacity-50"
            >
              Create
            </button>
            <button
              type="button"
              onClick={() => setShowCreate(false)}
              className="rounded-md border border-border px-4 py-2 text-sm text-muted-foreground transition-colors hover:text-foreground hover:bg-secondary"
            >
              Cancel
            </button>
          </div>
          {error && (
            <p className="mt-2 text-sm text-destructive">{error}</p>
          )}
        </form>
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
                  <th className="px-4 py-3 text-right font-medium text-muted-foreground">
                    Created
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
                    <td className="px-4 py-3 text-right text-muted-foreground">
                      {new Date(key.created_at).toLocaleDateString()}
                    </td>
                    <td className="px-4 py-3 text-right">
                      <div className="flex items-center justify-end gap-1">
                        {key.status === "enabled" && (
                          <button
                            onClick={() =>
                              handleStatusChange(key.id, "disable")
                            }
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
