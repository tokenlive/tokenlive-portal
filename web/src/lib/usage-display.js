export function formatTokens(value) {
  return Number(value || 0).toLocaleString();
}

export function formatCostCNY(value) {
  const text = String(value ?? "").trim();
  return text ? `¥${text}` : "-";
}

export function formatLatency(value) {
  const n = Number(value || 0);
  return n > 0 ? `${n.toLocaleString()} ms` : "-";
}

export function formatSuccessRate(success, total) {
  const s = Number(success || 0);
  const t = Number(total || 0);
  if (t <= 0) {
    return "-";
  }
  return `${((s / t) * 100).toFixed(1)}%`;
}
