// ─── API Response Types ───
// These mirror the backend Go structs

export interface ModelListItem {
  model_id: string;
  slug: string;
  status: string;
  display_name: string;
  short_description: string;
  tags?: string[];
  context_length?: number;
  input_modalities?: string[];
  output_modalities?: string[];
  capabilities?: string[];
  featured: boolean;
  price?: ModelPrice;
  metrics?: ModelMetrics;
  logo_url?: string;
}

export interface ModelPrice {
  currency: string;
  input_price: number;
  output_price: number;
  cached_price?: number;
  cache_creation_price?: number;
}

export interface ModelMetrics {
  window: string;
  availability?: number;
  ttft_p50_ms?: number;
  ttft_p95_ms?: number;
  response_speed?: number;
  success_rate?: number;
  sample_count: number;
  updated_at: string;
}

export interface ModelDetail {
  model_id: string;
  slug: string;
  status: string;
  display_name: string;
  short_description: string;
  tags?: string[];
  context_length?: number;
  input_modalities?: string[];
  output_modalities?: string[];
  capabilities?: string[];
  featured: boolean;
  price?: ModelPrice;
  metrics?: ModelMetrics;
  knowledge_cutoff?: string;
  logo_url?: string;
  long_description: string;
  seo_title: string;
  seo_description: string;
  service_metrics?: ModelMetrics[];
}

export interface ModelListResponse {
  data: ModelListItem[];
  pagination: ModelPagination;
}

export interface ModelPagination {
  limit: number;
  offset: number;
}

// ─── Auth Types ───

export interface CurrentUser {
  id: string;
  display_name: string;
  primary_email: string;
  email_verified: boolean;
  terms_accepted: boolean;
  avatar_url: string;
}

export interface StartEmailLoginResponse {
  sent: boolean;
  dev_code?: string;
}

export interface VerifyEmailLoginResponse {
  user: CurrentUser;
}

export interface OAuthLoginResult {
  user: CurrentUser;
  terms_pending: boolean;
}

export interface AcceptTermsResult {
  user: CurrentUser;
  workspace: WorkspaceResponse;
}

export interface WorkspaceResponse {
  id: string;
  name: string;
  slug: string;
  role: string;
  status: string;
  trial_granted_at?: string;
  balance: WorkspaceBalanceResponse;
}

export interface WorkspaceBalanceResponse {
  available_micro_cny: number;
  frozen_micro_cny: number;
  available_cny: string;
  frozen_cny: string;
}

export interface AccountIdentityDTO {
  provider: string;
  display_name: string;
  email: string;
  linked_at?: string;
}

// ─── Console Types ───

export interface ConsoleOverviewResponse {
  workspace: WorkspaceResponse;
  activation: ActivationOverviewResponse;
}

export interface BillingOverviewResponse {
  workspace: WorkspaceResponse;
  recharge_requests: RechargeRequestResponse[];
  ledger_entries: LedgerEntryResponse[];
}

export interface LedgerEntryResponse {
  id: string;
  type: "recharge" | "trial_grant" | "consumption" | "refund" | "adjustment";
  direction: "credit" | "debit";
  amount_micro_cny: number;
  amount_cny: string;
  balance_after_micro_cny: number;
  balance_after_cny: string;
  currency: string;
  api_key_id?: string;
  api_key_name_snapshot: string;
  model_id: string;
  model_display_name_snapshot: string;
  created_at: string;
}

export interface RechargeRequestResponse {
  id: string;
  requested_by_user_id: string;
  amount_micro_cny: number;
  amount_cny: string;
  currency: string;
  status: "pending" | "approved" | "rejected";
  payment_method: string;
  contact: string;
  note: string;
  admin_note: string;
  created_at: string;
  updated_at: string;
}

export interface ActivationOverviewResponse {
  trial_credit_granted: boolean;
  trial_expires_at?: string;
  api_key_created: boolean;
  runtime_activated: boolean;
  first_call_made: boolean;
  steps: ActivationStepResponse[];
}

export interface ActivationStepResponse {
  key: string;
  label: string;
  status: "completed" | "pending";
}

export interface APIKeyResponse {
  id: string;
  name: string;
  key_prefix: string;
  secret_last4: string;
  status: "enabled" | "disabled" | "revoked";
  expires_at?: string;
  daily_limit_micro_cny?: number;
  daily_limit_cny?: string;
  monthly_limit_micro_cny?: number;
  monthly_limit_cny?: string;
  last_used_at?: string;
  total_spend_micro_cny: number;
  total_spend_cny: string;
  created_at: string;
  updated_at: string;
}

export interface ListAPIKeysResponse {
  data: APIKeyResponse[];
}

export interface CreateAPIKeyResponse {
  api_key: APIKeyResponse;
  secret: string;
}

export interface CreateAPIKeyRequest {
  name: string;
  daily_limit_micro_cny?: number;
  monthly_limit_micro_cny?: number;
  expires_at?: string;
}

export interface CreateRechargeRequestRequest {
  amount_micro_cny: number;
  payment_method: string;
  contact: string;
  note: string;
}

export interface CreateRechargeRequestResponse {
  recharge_request: RechargeRequestResponse;
}
