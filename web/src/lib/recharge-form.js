const MICRO_CNY_PER_CNY = 1_000_000;

export function buildCreateRechargeRequest({
  amountCNY,
  paymentMethod,
  contact,
  note,
}) {
  return {
    amount_micro_cny: parseRequiredCNYToMicroCNY(amountCNY),
    payment_method: String(paymentMethod ?? "").trim(),
    contact: String(contact ?? "").trim(),
    note: String(note ?? "").trim(),
  };
}

export function formatRechargeStatus(status) {
  const labels = {
    pending: "Pending",
    approved: "Approved",
    rejected: "Rejected",
  };
  return labels[status] || "Unknown";
}

export function formatLedgerType(type) {
  const labels = {
    recharge: "Recharge",
    trial_grant: "Trial credit",
    consumption: "Usage",
    refund: "Refund",
    adjustment: "Adjustment",
  };
  return labels[type] || "Unknown";
}

export function formatLedgerDirection(direction) {
  if (direction === "credit") {
    return "+";
  }
  if (direction === "debit") {
    return "-";
  }
  return "";
}

function parseRequiredCNYToMicroCNY(value) {
  const trimmed = String(value ?? "").trim();
  if (trimmed === "") {
    return 0;
  }
  return Math.round(Number(trimmed) * MICRO_CNY_PER_CNY);
}
