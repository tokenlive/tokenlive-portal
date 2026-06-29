export function buildModelDetailHref(model) {
  const id = model?.slug || model?.model_id || "";
  return `/models/${encodeURIComponent(id)}`;
}

export function formatModelPrice(price) {
  if (price === undefined || price === null || Number.isNaN(Number(price))) {
    return "-";
  }
  return `¥${Number(price).toFixed(2)}/1M`;
}

export function formatPercentMetric(value) {
  if (value === undefined || value === null || Number.isNaN(Number(value))) {
    return "-";
  }
  const percent = Number(value) <= 1 ? Number(value) * 100 : Number(value);
  return `${percent.toFixed(1)}%`;
}

export function formatThroughput(value) {
  if (value === undefined || value === null || Number.isNaN(Number(value))) {
    return "-";
  }
  return `${Number(value).toFixed(1)} tok/s`;
}
