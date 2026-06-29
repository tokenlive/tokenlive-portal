"use client";

import { useCallback, useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import { createRechargeRequest, fetchBillingOverview } from "@/lib/api";
import {
  buildCreateRechargeRequest,
  formatLedgerDirection,
  formatLedgerType,
  formatRechargeStatus,
} from "@/lib/recharge-form";
import { getConsoleAuthRedirect } from "@/lib/auth-flow";
import type {
  BillingOverviewResponse,
  LedgerEntryResponse,
  RechargeRequestResponse,
} from "@/types/api";

export default function BillingPage() {
  const router = useRouter();
  const [overview, setOverview] = useState<BillingOverviewResponse | null>(null);
  const [loading, setLoading] = useState(true);
  const [submitting, setSubmitting] = useState(false);
  const [amountCNY, setAmountCNY] = useState("");
  const [paymentMethod, setPaymentMethod] = useState("bank_transfer");
  const [contact, setContact] = useState("");
  const [note, setNote] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [notice, setNotice] = useState<string | null>(null);

  const loadOverview = useCallback(async () => {
    try {
      const res = await fetchBillingOverview();
      setOverview(res);
    } catch (err) {
      const redirectTo = getConsoleAuthRedirect(err);
      if (redirectTo) {
        router.replace(redirectTo);
        return;
      }
      setError(err instanceof Error ? err.message : "Failed to load billing");
    } finally {
      setLoading(false);
    }
  }, [router]);

  useEffect(() => {
    void Promise.resolve().then(loadOverview);
  }, [loadOverview]);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setSubmitting(true);
    setError(null);
    setNotice(null);
    try {
      await createRechargeRequest(
        buildCreateRechargeRequest({
          amountCNY,
          paymentMethod,
          contact,
          note,
        })
      );
      setAmountCNY("");
      setNote("");
      setNotice("Recharge request submitted");
      await loadOverview();
    } catch (err) {
      const redirectTo = getConsoleAuthRedirect(err);
      if (redirectTo) {
        router.replace(redirectTo);
        return;
      }
      setError(
        err instanceof Error ? err.message : "Failed to submit recharge request"
      );
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <div className="p-6 lg:p-8">
      <div className="mb-8">
        <div className="mb-1 flex items-center gap-2">
          <span className="tl-node tl-node--active" />
          <h1 className="font-heading text-xl font-semibold text-foreground">
            Billing
          </h1>
        </div>
        <p className="text-sm text-muted-foreground">
          Review balance and submit manual recharge requests.
        </p>
      </div>

      {error && (
        <div className="mb-6 rounded-lg border border-destructive/30 bg-destructive/10 p-4 text-sm text-destructive">
          {error}
        </div>
      )}
      {notice && (
        <div className="mb-6 rounded-lg border border-primary/30 bg-primary/5 p-4 text-sm text-primary">
          {notice}
        </div>
      )}

      {loading ? (
        <div className="py-12 text-center text-sm text-muted-foreground">
          Loading billing...
        </div>
      ) : (
        <div className="grid gap-6 xl:grid-cols-[380px_1fr]">
          <div className="space-y-6">
            <section className="rounded-lg border border-border bg-card p-5">
              <p className="mb-2 text-sm font-medium text-muted-foreground">
                Available balance
              </p>
              <p className="font-heading text-3xl font-semibold text-foreground">
                ¥{overview?.workspace.balance.available_cny ?? "0.000000"}
              </p>
              <div className="mt-4 grid grid-cols-2 gap-3 text-sm">
                <Metric
                  label="Frozen"
                  value={`¥${overview?.workspace.balance.frozen_cny ?? "0.000000"}`}
                />
                <Metric label="Role" value={overview?.workspace.role ?? "—"} />
              </div>
            </section>

            <form
              onSubmit={handleSubmit}
              className="rounded-lg border border-border bg-card p-5"
            >
              <h2 className="mb-4 font-heading text-sm font-medium uppercase tracking-wider text-muted-foreground">
                New Recharge
              </h2>
              <div className="space-y-4">
                <Field label="Amount" htmlFor="recharge-amount">
                  <MoneyInput
                    id="recharge-amount"
                    value={amountCNY}
                    onChange={setAmountCNY}
                  />
                </Field>
                <Field label="Payment method" htmlFor="payment-method">
                  <select
                    id="payment-method"
                    value={paymentMethod}
                    onChange={(e) => setPaymentMethod(e.target.value)}
                    className="h-10 w-full rounded-md border border-input bg-background px-3 text-sm text-foreground focus:outline-none focus:ring-2 focus:ring-primary/40"
                  >
                    <option value="bank_transfer">Bank transfer</option>
                    <option value="alipay">Alipay</option>
                    <option value="wechat_pay">WeChat Pay</option>
                    <option value="other">Other</option>
                  </select>
                </Field>
                <Field label="Contact" htmlFor="recharge-contact">
                  <input
                    id="recharge-contact"
                    type="text"
                    required
                    value={contact}
                    onChange={(e) => setContact(e.target.value)}
                    placeholder="ops@example.com"
                    className="h-10 w-full rounded-md border border-input bg-background px-3 text-sm text-foreground placeholder:text-muted-foreground focus:outline-none focus:ring-2 focus:ring-primary/40"
                  />
                </Field>
                <Field label="Note" htmlFor="recharge-note">
                  <textarea
                    id="recharge-note"
                    value={note}
                    onChange={(e) => setNote(e.target.value)}
                    rows={4}
                    placeholder="Invoice title, transfer reference, or internal note"
                    className="w-full resize-none rounded-md border border-input bg-background px-3 py-2 text-sm text-foreground placeholder:text-muted-foreground focus:outline-none focus:ring-2 focus:ring-primary/40"
                  />
                </Field>
              </div>
              <button
                type="submit"
                disabled={submitting || !amountCNY.trim() || !contact.trim()}
                className="mt-5 w-full rounded-md bg-primary px-4 py-2 text-sm font-medium text-primary-foreground transition-colors hover:bg-primary/90 disabled:opacity-50"
              >
                {submitting ? "Submitting..." : "Submit request"}
              </button>
            </form>
          </div>

          <div className="space-y-6">
            <section className="rounded-lg border border-border bg-card">
              <div className="border-b border-border px-5 py-4">
                <h2 className="font-heading text-sm font-medium uppercase tracking-wider text-muted-foreground">
                  Balance Activity
                </h2>
              </div>
              {overview?.ledger_entries.length ? (
                <div className="overflow-x-auto">
                  <table className="w-full text-sm">
                    <thead>
                      <tr className="border-b border-border bg-muted/30">
                        <th className="px-4 py-3 text-left font-medium text-muted-foreground">
                          Type
                        </th>
                        <th className="px-4 py-3 text-left font-medium text-muted-foreground">
                          Detail
                        </th>
                        <th className="px-4 py-3 text-right font-medium text-muted-foreground">
                          Amount
                        </th>
                        <th className="px-4 py-3 text-right font-medium text-muted-foreground">
                          Balance
                        </th>
                        <th className="px-4 py-3 text-right font-medium text-muted-foreground">
                          Created
                        </th>
                      </tr>
                    </thead>
                    <tbody>
                      {overview.ledger_entries.map((entry) => (
                        <LedgerRow key={entry.id} entry={entry} />
                      ))}
                    </tbody>
                  </table>
                </div>
              ) : (
                <div className="px-5 py-12 text-center text-sm text-muted-foreground">
                  No balance activity yet.
                </div>
              )}
            </section>

            <section className="rounded-lg border border-border bg-card">
              <div className="border-b border-border px-5 py-4">
                <h2 className="font-heading text-sm font-medium uppercase tracking-wider text-muted-foreground">
                  Recent Requests
                </h2>
              </div>
              {overview?.recharge_requests.length ? (
                <div className="overflow-x-auto">
                  <table className="w-full text-sm">
                    <thead>
                      <tr className="border-b border-border bg-muted/30">
                        <th className="px-4 py-3 text-left font-medium text-muted-foreground">
                          Amount
                        </th>
                        <th className="px-4 py-3 text-left font-medium text-muted-foreground">
                          Method
                        </th>
                        <th className="px-4 py-3 text-left font-medium text-muted-foreground">
                          Status
                        </th>
                        <th className="px-4 py-3 text-left font-medium text-muted-foreground">
                          Contact
                        </th>
                        <th className="px-4 py-3 text-right font-medium text-muted-foreground">
                          Created
                        </th>
                      </tr>
                    </thead>
                    <tbody>
                      {overview.recharge_requests.map((request) => (
                        <RechargeRow key={request.id} request={request} />
                      ))}
                    </tbody>
                  </table>
                </div>
              ) : (
                <div className="px-5 py-12 text-center text-sm text-muted-foreground">
                  No recharge requests yet.
                </div>
              )}
            </section>
          </div>
        </div>
      )}
    </div>
  );
}

function Metric({ label, value }: { label: string; value: string }) {
  return (
    <div className="rounded-md bg-muted/30 px-3 py-2">
      <p className="text-xs text-muted-foreground">{label}</p>
      <p className="mt-1 truncate text-sm font-medium text-foreground">{value}</p>
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
}: {
  id: string;
  value: string;
  onChange: (value: string) => void;
}) {
  return (
    <div className="relative">
      <span className="pointer-events-none absolute left-3 top-1/2 -translate-y-1/2 text-sm text-muted-foreground">
        ¥
      </span>
      <input
        id={id}
        type="number"
        min="0.01"
        step="0.01"
        required
        value={value}
        onChange={(e) => onChange(e.target.value)}
        placeholder="100.00"
        className="h-10 w-full rounded-md border border-input bg-background pl-7 pr-3 text-sm text-foreground placeholder:text-muted-foreground focus:outline-none focus:ring-2 focus:ring-primary/40"
      />
    </div>
  );
}

function RechargeRow({ request }: { request: RechargeRequestResponse }) {
  return (
    <tr className="border-b border-border last:border-b-0 hover:bg-muted/20">
      <td className="px-4 py-3 font-medium text-foreground">
        ¥{request.amount_cny}
      </td>
      <td className="px-4 py-3 text-muted-foreground">
        {formatPaymentMethod(request.payment_method)}
      </td>
      <td className="px-4 py-3">
        <RechargeStatusBadge status={request.status} />
      </td>
      <td className="px-4 py-3 text-muted-foreground">{request.contact}</td>
      <td className="px-4 py-3 text-right text-muted-foreground">
        {new Date(request.created_at).toLocaleDateString()}
      </td>
    </tr>
  );
}

function LedgerRow({ entry }: { entry: LedgerEntryResponse }) {
  const sign = formatLedgerDirection(entry.direction);
  const detail =
    entry.model_display_name_snapshot ||
    entry.api_key_name_snapshot ||
    entry.model_id ||
    "Workspace balance";
  const amountTone =
    entry.direction === "credit" ? "text-green-400" : "text-foreground";

  return (
    <tr className="border-b border-border last:border-b-0 hover:bg-muted/20">
      <td className="px-4 py-3 font-medium text-foreground">
        {formatLedgerType(entry.type)}
      </td>
      <td className="px-4 py-3 text-muted-foreground">{detail}</td>
      <td className={`px-4 py-3 text-right font-medium ${amountTone}`}>
        {sign}¥{entry.amount_cny}
      </td>
      <td className="px-4 py-3 text-right text-muted-foreground">
        ¥{entry.balance_after_cny}
      </td>
      <td className="px-4 py-3 text-right text-muted-foreground">
        {new Date(entry.created_at).toLocaleDateString()}
      </td>
    </tr>
  );
}

function RechargeStatusBadge({ status }: { status: string }) {
  const variants: Record<string, { dot: string; text: string }> = {
    pending: { dot: "bg-amber-400", text: "text-amber-400" },
    approved: { dot: "bg-green-500", text: "text-green-400" },
    rejected: { dot: "bg-destructive", text: "text-destructive" },
  };
  const v = variants[status] || { dot: "bg-muted-foreground", text: "text-muted-foreground" };
  return (
    <span className={`inline-flex items-center gap-1.5 text-xs ${v.text}`}>
      <span className={`h-1.5 w-1.5 rounded-full ${v.dot}`} />
      {formatRechargeStatus(status)}
    </span>
  );
}

function formatPaymentMethod(method: string) {
  const labels: Record<string, string> = {
    bank_transfer: "Bank transfer",
    alipay: "Alipay",
    wechat_pay: "WeChat Pay",
    other: "Other",
  };
  return labels[method] || method;
}
