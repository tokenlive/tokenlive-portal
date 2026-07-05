import type {
  ModelListResponse,
  ModelDetail,
  CurrentUser,
  StartEmailLoginResponse,
  VerifyEmailLoginResponse,
  AcceptTermsResult,
  ConsoleOverviewResponse,
  BillingOverviewResponse,
  UsageSummaryResponse,
  RequestLogsResponse,
  ListAPIKeysResponse,
  CreateAPIKeyResponse,
  CreateAPIKeyRequest,
  CreateRechargeRequestRequest,
  CreateRechargeRequestResponse,
  APIKeyResponse,
  AccountIdentityDTO,
} from "@/types/api";

const API_BASE = process.env.NEXT_PUBLIC_API_URL || "";

// ─── HTTP helpers ───

async function request<T>(
  path: string,
  init?: RequestInit
): Promise<T> {
  const res = await fetch(`${API_BASE}${path}`, {
    credentials: "include", // always send cookies for session auth
    headers: {
      "Content-Type": "application/json",
      ...init?.headers,
    },
    ...init,
  });

  if (!res.ok) {
    const body = await res.json().catch(() => ({}));
    const error = new Error(
      (body as { error?: { message?: string } })?.error?.message ||
        `HTTP ${res.status}`
    );
    (error as Error & { status: number }).status = res.status;
    (error as Error & { code: string }).code =
      (body as { error?: { code?: string } })?.error?.code || "unknown";
    throw error;
  }

  return res.json();
}

// ─── Public API ───

export async function fetchModels(): Promise<ModelListResponse> {
  return request<ModelListResponse>("/api/public/models");
}

export async function fetchModelDetail(slug: string): Promise<ModelDetail> {
  const res = await request<{ data: ModelDetail }>(`/api/public/models/${slug}`);
  return res.data;
}

// ─── Auth API ───

export async function startEmailLogin(
  email: string
): Promise<StartEmailLoginResponse> {
  return request<StartEmailLoginResponse>("/api/auth/email/start", {
    method: "POST",
    body: JSON.stringify({ email }),
  });
}

export async function verifyEmailLogin(
  email: string,
  code: string
): Promise<VerifyEmailLoginResponse> {
  return request<VerifyEmailLoginResponse>("/api/auth/email/verify", {
    method: "POST",
    body: JSON.stringify({ email, code }),
  });
}

export async function getMe(): Promise<CurrentUser> {
  const res = await request<{ user: CurrentUser }>("/api/me");
  return res.user;
}

export async function logout(): Promise<void> {
  await request("/api/auth/logout", { method: "POST" });
}

export async function acceptTerms(): Promise<AcceptTermsResult> {
  return request<AcceptTermsResult>("/api/auth/accept-terms", { method: "POST" });
}

export async function listOAuthAccounts(): Promise<AccountIdentityDTO[]> {
  const res = await request<{ accounts: AccountIdentityDTO[] }>(
    "/api/auth/oauth/accounts"
  );
  return res.accounts;
}

// ─── Console API ───

export async function fetchOverview(): Promise<ConsoleOverviewResponse> {
  return request<ConsoleOverviewResponse>("/api/console/overview");
}

export async function fetchBillingOverview(): Promise<BillingOverviewResponse> {
  return request<BillingOverviewResponse>("/api/billing/overview");
}

export async function fetchUsageSummary(): Promise<UsageSummaryResponse> {
  return request<UsageSummaryResponse>("/api/usage/summary");
}

export async function fetchRequestLogs(limit = 50): Promise<RequestLogsResponse> {
  return request<RequestLogsResponse>(`/api/request-logs?limit=${limit}`);
}

export async function createRechargeRequest(
  data: CreateRechargeRequestRequest
): Promise<CreateRechargeRequestResponse> {
  return request<CreateRechargeRequestResponse>("/api/billing/recharge-requests", {
    method: "POST",
    body: JSON.stringify(data),
  });
}

export async function listAPIKeys(): Promise<ListAPIKeysResponse> {
  return request<ListAPIKeysResponse>("/api/api-keys");
}

export async function createAPIKey(
  data: CreateAPIKeyRequest
): Promise<CreateAPIKeyResponse> {
  return request<CreateAPIKeyResponse>("/api/api-keys", {
    method: "POST",
    body: JSON.stringify(data),
  });
}

export async function updateAPIKeyStatus(
  id: string,
  action: "enable" | "disable" | "revoke"
): Promise<APIKeyResponse> {
  return request<APIKeyResponse>(`/api/api-keys/${id}/${action}`, { method: "POST" });
}

// ─── OAuth URLs (browser redirects) ───

export const GOOGLE_LOGIN_URL = `${API_BASE}/api/auth/google/login`;
export const GOOGLE_BIND_URL = `${API_BASE}/api/auth/google/bind`;
