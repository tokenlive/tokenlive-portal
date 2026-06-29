const API_BASE = process.env.NEXT_PUBLIC_API_URL || "";

export const OAUTH_PROVIDERS = [
  {
    id: "google",
    label: "Google",
    loginHref: `${API_BASE}/api/auth/google/login`,
    bindHref: `${API_BASE}/api/auth/google/bind`,
    enabled: true,
    unavailableLabel: null,
  },
  {
    id: "github",
    label: "GitHub",
    loginHref: `${API_BASE}/api/auth/github/login`,
    bindHref: `${API_BASE}/api/auth/github/bind`,
    enabled: true,
    unavailableLabel: null,
  },
];

export function getOAuthProviderRows(accounts = []) {
  return OAUTH_PROVIDERS.map((provider) => {
    const account = accounts.find((item) => item.provider === provider.id) || null;
    return {
      ...provider,
      account,
      connected: account !== null,
      disabled: !provider.enabled,
    };
  });
}
