const MICRO_CNY_PER_CNY = 1_000_000;

export function buildCreateAPIKeyRequest({
  name,
  dailyLimitCNY,
  monthlyLimitCNY,
  expiresOn,
}) {
  const request = {
    name: name.trim(),
  };

  const dailyLimit = parseCNYToMicroCNY(dailyLimitCNY);
  if (dailyLimit !== undefined) {
    request.daily_limit_micro_cny = dailyLimit;
  }

  const monthlyLimit = parseCNYToMicroCNY(monthlyLimitCNY);
  if (monthlyLimit !== undefined) {
    request.monthly_limit_micro_cny = monthlyLimit;
  }

  if (expiresOn) {
    request.expires_at = `${expiresOn}T23:59:59.000Z`;
  }

  return request;
}

export function formatAPIKeyLimit(microCNY) {
  if (microCNY === undefined || microCNY === null) {
    return "Unlimited";
  }
  return `¥${(microCNY / MICRO_CNY_PER_CNY).toFixed(2)}`;
}

function parseCNYToMicroCNY(value) {
  const trimmed = String(value ?? "").trim();
  if (trimmed === "") {
    return undefined;
  }
  return Math.round(Number(trimmed) * MICRO_CNY_PER_CNY);
}
