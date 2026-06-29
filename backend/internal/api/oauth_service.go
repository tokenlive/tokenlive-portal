package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/tokenlive/tokenlive-portal/backend/internal/domain"
	"github.com/tokenlive/tokenlive-portal/backend/internal/repository"
	"github.com/tokenlive/tokenlive-portal/backend/internal/security"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"gorm.io/gorm"
)

const (
	googleProviderName = "google"
	githubProviderName = "github"
	googleUserInfoURL  = "https://www.googleapis.com/oauth2/v3/userinfo"
	githubUserInfoURL  = "https://api.github.com/user"
	githubEmailsURL    = "https://api.github.com/user/emails"
	oauthHTTPTimeout   = 10 * time.Second
)

var (
	errOAuthExchangeFailed = errors.New("oauth token exchange failed")
	errOAuthUserInfoFailed = errors.New("oauth userinfo fetch failed")
)

type googleUserInfo struct {
	Sub           string `json:"sub"`
	Email         string `json:"email"`
	EmailVerified bool   `json:"email_verified"`
	Name          string `json:"name"`
	Picture       string `json:"picture"`
}

type githubUserInfo struct {
	ID        int64  `json:"id"`
	Login     string `json:"login"`
	Name      string `json:"name"`
	Email     string `json:"email"`
	AvatarURL string `json:"avatar_url"`
}

type githubEmailInfo struct {
	Email    string `json:"email"`
	Primary  bool   `json:"primary"`
	Verified bool   `json:"verified"`
}

type oauthProfile struct {
	Provider        string
	ProviderSubject string
	Email           string
	EmailVerified   bool
	DisplayName     string
	AvatarURL       string
}

func (s *authService) GetGoogleAuthURL(state string) string {
	cfg := s.googleOAuthConfig()
	if cfg == nil {
		return ""
	}
	return cfg.AuthCodeURL(state, oauth2.AccessTypeOnline)
}

func (s *authService) googleOAuthConfig() *oauth2.Config {
	if !s.googleOAuth.Enabled() {
		return nil
	}
	return &oauth2.Config{
		ClientID:     s.googleOAuth.ClientID,
		ClientSecret: s.googleOAuth.ClientSecret,
		RedirectURL:  s.googleOAuth.RedirectURL,
		Scopes:       []string{"openid", "email", "profile"},
		Endpoint:     google.Endpoint,
	}
}

func (s *authService) GetGitHubAuthURL(state string) string {
	cfg := s.githubOAuthConfig()
	if cfg == nil {
		return ""
	}
	return cfg.AuthCodeURL(state)
}

func (s *authService) githubOAuthConfig() *oauth2.Config {
	if !s.githubOAuth.Enabled() {
		return nil
	}
	return &oauth2.Config{
		ClientID:     s.githubOAuth.ClientID,
		ClientSecret: s.githubOAuth.ClientSecret,
		RedirectURL:  s.githubOAuth.RedirectURL,
		Scopes:       []string{"read:user", "user:email"},
		Endpoint: oauth2.Endpoint{
			AuthURL:  "https://github.com/login/oauth/authorize",
			TokenURL: "https://github.com/login/oauth/access_token",
		},
	}
}

func (s *authService) HandleGoogleCallback(ctx context.Context, code, ip, userAgent string) (OAuthLoginResult, error) {
	cfg := s.googleOAuthConfig()
	if cfg == nil {
		return OAuthLoginResult{}, ErrAuthOAuthNotConfigured
	}

	code = strings.TrimSpace(code)
	if code == "" {
		return OAuthLoginResult{}, ErrAuthInvalidRequest
	}

	httpClient := &http.Client{Timeout: oauthHTTPTimeout}
	tokenCtx := context.WithValue(ctx, oauth2.HTTPClient, httpClient)
	token, err := cfg.Exchange(tokenCtx, code)
	if err != nil {
		return OAuthLoginResult{}, fmt.Errorf("%w: %v", errOAuthExchangeFailed, err)
	}

	userInfo, err := fetchGoogleUserInfo(httpClient, token.AccessToken)
	if err != nil {
		return OAuthLoginResult{}, err
	}

	return s.handleOAuthLogin(ctx, oauthProfile{
		Provider:        googleProviderName,
		ProviderSubject: userInfo.Sub,
		Email:           userInfo.Email,
		EmailVerified:   userInfo.EmailVerified,
		DisplayName:     userInfo.Name,
		AvatarURL:       userInfo.Picture,
	}, ip, userAgent)
}

func (s *authService) HandleGitHubCallback(ctx context.Context, code, ip, userAgent string) (OAuthLoginResult, error) {
	cfg := s.githubOAuthConfig()
	if cfg == nil {
		return OAuthLoginResult{}, ErrAuthOAuthNotConfigured
	}

	code = strings.TrimSpace(code)
	if code == "" {
		return OAuthLoginResult{}, ErrAuthInvalidRequest
	}

	httpClient := &http.Client{Timeout: oauthHTTPTimeout}
	tokenCtx := context.WithValue(ctx, oauth2.HTTPClient, httpClient)
	token, err := cfg.Exchange(tokenCtx, code)
	if err != nil {
		return OAuthLoginResult{}, fmt.Errorf("%w: %v", errOAuthExchangeFailed, err)
	}

	profile, err := fetchGitHubProfile(httpClient, token.AccessToken)
	if err != nil {
		return OAuthLoginResult{}, err
	}

	return s.handleOAuthLogin(ctx, profile, ip, userAgent)
}

func (s *authService) handleOAuthLogin(ctx context.Context, profile oauthProfile, ip, userAgent string) (OAuthLoginResult, error) {
	if strings.TrimSpace(profile.ProviderSubject) == "" {
		return OAuthLoginResult{}, fmt.Errorf("%w: missing subject field", errOAuthUserInfoFailed)
	}

	email := strings.ToLower(strings.TrimSpace(profile.Email))
	displayName := strings.TrimSpace(profile.DisplayName)
	if displayName == "" {
		displayName = workspaceNameFromEmail(email)
	}

	identity, err := s.store.FindAccountIdentityByProviderSubject(ctx, profile.Provider, profile.ProviderSubject)
	if err != nil && !errors.Is(err, repository.ErrAccountIdentityNotFound) {
		return OAuthLoginResult{}, err
	}

	var user domain.User
	termsPending := false

	if errors.Is(err, repository.ErrAccountIdentityNotFound) {
		if email != "" {
			_, findErr := s.store.FindUserByPrimaryEmail(ctx, email)
			if findErr == nil {
				return OAuthLoginResult{}, ErrAuthOAuthEmailConflict
			}
			if findErr != nil && !isRecordNotFound(findErr) {
				return OAuthLoginResult{}, findErr
			}
		}

		created, err := s.store.CreateOAuthUser(ctx, repository.CreateOAuthUserInput{
			Provider:        profile.Provider,
			ProviderSubject: profile.ProviderSubject,
			Email:           email,
			EmailVerified:   profile.EmailVerified,
			DisplayName:     displayName,
			AvatarURL:       profile.AvatarURL,
		})
		if err != nil {
			if errors.Is(err, repository.ErrUserEmailExists) {
				return OAuthLoginResult{}, ErrAuthOAuthEmailConflict
			}
			if errors.Is(err, repository.ErrIdentityAlreadyBound) {
				return OAuthLoginResult{}, ErrAuthOAuthIdentityBound
			}
			return OAuthLoginResult{}, err
		}
		user = created.User
		termsPending = user.TermsAcceptedAt == nil
	} else {
		user, err = s.store.FindUserByID(ctx, identity.UserID)
		if err != nil {
			return OAuthLoginResult{}, err
		}
		termsPending = user.TermsAcceptedAt == nil
	}

	sessionToken, err := s.generateSessionToken()
	if err != nil {
		return OAuthLoginResult{}, fmt.Errorf("generate session token: %w", err)
	}

	now := s.nowFunc().UTC()
	_, err = s.store.CreateSession(ctx, repository.CreateSessionInput{
		UserID:    user.ID,
		TokenHash: security.HashSecret(sessionToken, s.authPepper),
		IP:        ip,
		UserAgent: userAgent,
		ExpiresAt: now.Add(sessionCookieTTL),
	})
	if err != nil {
		return OAuthLoginResult{}, fmt.Errorf("create session: %w", err)
	}

	return OAuthLoginResult{
		SessionToken: sessionToken,
		User:         currentUserFromDomain(user),
		TermsPending: termsPending,
	}, nil
}

func (s *authService) AcceptTerms(ctx context.Context, sessionToken string) (AcceptTermsResult, error) {
	session, err := s.resolveActiveSession(ctx, sessionToken)
	if err != nil {
		return AcceptTermsResult{}, err
	}

	user, err := s.store.FindUserByID(ctx, session.UserID)
	if err != nil {
		return AcceptTermsResult{}, err
	}
	if user.TermsAcceptedAt != nil {
		return AcceptTermsResult{}, ErrAuthTermsAlreadyAccepted
	}

	slugSuffix, err := s.generateSlugSuffix()
	if err != nil {
		return AcceptTermsResult{}, fmt.Errorf("generate workspace slug suffix: %w", err)
	}

	workspaceName := user.DisplayName
	if workspaceName == "" {
		workspaceName = workspaceNameFromEmail(derefString(user.PrimaryEmail))
	}

	completed, err := s.store.CompleteUserOnboarding(ctx, repository.CompleteUserOnboardingInput{
		UserID:        user.ID,
		WorkspaceName: workspaceName,
		WorkspaceSlug: "personal-" + slugSuffix,
		TrialCredit: repository.TrialCreditInput{
			AmountMicroCNY: s.trialCredit.AmountMicroCNY,
			TTLDays:        s.trialCredit.TTLDays,
			Source:         "oauth_registration",
		},
	})
	if err != nil {
		return AcceptTermsResult{}, err
	}

	updatedUser, err := s.store.FindUserByID(ctx, user.ID)
	if err != nil {
		return AcceptTermsResult{}, err
	}

	return AcceptTermsResult{
		User: currentUserFromDomain(updatedUser),
		Workspace: WorkspaceDTO{
			ID:   completed.Workspace.ID,
			Name: completed.Workspace.Name,
			Slug: completed.Workspace.Slug,
		},
	}, nil
}

func (s *authService) HandleGoogleBind(ctx context.Context, sessionToken, code string) (AccountIdentityDTO, error) {
	session, err := s.resolveActiveSession(ctx, sessionToken)
	if err != nil {
		return AccountIdentityDTO{}, err
	}

	cfg := s.googleOAuthConfig()
	if cfg == nil {
		return AccountIdentityDTO{}, ErrAuthOAuthNotConfigured
	}

	code = strings.TrimSpace(code)
	if code == "" {
		return AccountIdentityDTO{}, ErrAuthInvalidRequest
	}

	httpClient := &http.Client{Timeout: oauthHTTPTimeout}
	tokenCtx := context.WithValue(ctx, oauth2.HTTPClient, httpClient)
	token, err := cfg.Exchange(tokenCtx, code)
	if err != nil {
		return AccountIdentityDTO{}, fmt.Errorf("%w: %v", errOAuthExchangeFailed, err)
	}

	userInfo, err := fetchGoogleUserInfo(httpClient, token.AccessToken)
	if err != nil {
		return AccountIdentityDTO{}, err
	}

	return s.linkOAuthIdentity(ctx, session.UserID, oauthProfile{
		Provider:        googleProviderName,
		ProviderSubject: userInfo.Sub,
		Email:           userInfo.Email,
		EmailVerified:   userInfo.EmailVerified,
		DisplayName:     userInfo.Name,
		AvatarURL:       userInfo.Picture,
	})
}

func (s *authService) HandleGitHubBind(ctx context.Context, sessionToken, code string) (AccountIdentityDTO, error) {
	session, err := s.resolveActiveSession(ctx, sessionToken)
	if err != nil {
		return AccountIdentityDTO{}, err
	}

	cfg := s.githubOAuthConfig()
	if cfg == nil {
		return AccountIdentityDTO{}, ErrAuthOAuthNotConfigured
	}

	code = strings.TrimSpace(code)
	if code == "" {
		return AccountIdentityDTO{}, ErrAuthInvalidRequest
	}

	httpClient := &http.Client{Timeout: oauthHTTPTimeout}
	tokenCtx := context.WithValue(ctx, oauth2.HTTPClient, httpClient)
	token, err := cfg.Exchange(tokenCtx, code)
	if err != nil {
		return AccountIdentityDTO{}, fmt.Errorf("%w: %v", errOAuthExchangeFailed, err)
	}

	profile, err := fetchGitHubProfile(httpClient, token.AccessToken)
	if err != nil {
		return AccountIdentityDTO{}, err
	}

	return s.linkOAuthIdentity(ctx, session.UserID, profile)
}

func (s *authService) linkOAuthIdentity(ctx context.Context, userID string, profile oauthProfile) (AccountIdentityDTO, error) {
	identity, err := s.store.LinkOAuthIdentity(ctx, repository.LinkOAuthIdentityInput{
		UserID:          userID,
		Provider:        profile.Provider,
		ProviderSubject: profile.ProviderSubject,
		Email:           strings.ToLower(strings.TrimSpace(profile.Email)),
		EmailVerified:   profile.EmailVerified,
		DisplayName:     strings.TrimSpace(profile.DisplayName),
		AvatarURL:       profile.AvatarURL,
	})
	if err != nil {
		if errors.Is(err, repository.ErrIdentityAlreadyBound) || errors.Is(err, repository.ErrUserAlreadyHasProvider) {
			return AccountIdentityDTO{}, ErrAuthOAuthIdentityBound
		}
		return AccountIdentityDTO{}, err
	}

	linkedAt := ""
	if identity.LinkedAt != nil {
		linkedAt = identity.LinkedAt.Format(time.RFC3339)
	}

	return AccountIdentityDTO{
		Provider:    identity.Provider,
		DisplayName: identity.DisplayName,
		Email:       identity.Email,
		LinkedAt:    linkedAt,
	}, nil
}

func (s *authService) ListAccountIdentities(ctx context.Context, sessionToken string) ([]AccountIdentityDTO, error) {
	session, err := s.resolveActiveSession(ctx, sessionToken)
	if err != nil {
		return nil, err
	}

	identities, err := s.store.ListAccountIdentitiesByUserID(ctx, session.UserID)
	if err != nil {
		return nil, err
	}

	result := make([]AccountIdentityDTO, 0, len(identities))
	for _, identity := range identities {
		linkedAt := ""
		if identity.LinkedAt != nil {
			linkedAt = identity.LinkedAt.Format(time.RFC3339)
		}
		result = append(result, AccountIdentityDTO{
			Provider:    identity.Provider,
			DisplayName: identity.DisplayName,
			Email:       identity.Email,
			LinkedAt:    linkedAt,
		})
	}
	return result, nil
}

func fetchGoogleUserInfo(client *http.Client, accessToken string) (*googleUserInfo, error) {
	req, err := http.NewRequest(http.MethodGet, googleUserInfoURL, nil)
	if err != nil {
		return nil, fmt.Errorf("build userinfo request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", errOAuthUserInfoFailed, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read userinfo response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%w: status %d", errOAuthUserInfoFailed, resp.StatusCode)
	}

	var info googleUserInfo
	if err := json.Unmarshal(body, &info); err != nil {
		return nil, fmt.Errorf("parse userinfo response: %w", err)
	}
	if info.Sub == "" {
		return nil, fmt.Errorf("%w: missing sub field", errOAuthUserInfoFailed)
	}
	return &info, nil
}

func fetchGitHubProfile(client *http.Client, accessToken string) (oauthProfile, error) {
	req, err := http.NewRequest(http.MethodGet, githubUserInfoURL, nil)
	if err != nil {
		return oauthProfile{}, fmt.Errorf("build userinfo request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := client.Do(req)
	if err != nil {
		return oauthProfile{}, fmt.Errorf("%w: %v", errOAuthUserInfoFailed, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return oauthProfile{}, fmt.Errorf("read userinfo response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return oauthProfile{}, fmt.Errorf("%w: status %d", errOAuthUserInfoFailed, resp.StatusCode)
	}

	var info githubUserInfo
	if err := json.Unmarshal(body, &info); err != nil {
		return oauthProfile{}, fmt.Errorf("parse userinfo response: %w", err)
	}
	if info.ID == 0 {
		return oauthProfile{}, fmt.Errorf("%w: missing id field", errOAuthUserInfoFailed)
	}

	email := strings.ToLower(strings.TrimSpace(info.Email))
	emailVerified := email != ""
	if email == "" {
		var err error
		email, emailVerified, err = fetchGitHubPrimaryEmail(client, accessToken)
		if err != nil {
			return oauthProfile{}, err
		}
	}

	displayName := strings.TrimSpace(info.Name)
	if displayName == "" {
		displayName = strings.TrimSpace(info.Login)
	}

	return oauthProfile{
		Provider:        githubProviderName,
		ProviderSubject: fmt.Sprintf("%d", info.ID),
		Email:           email,
		EmailVerified:   emailVerified,
		DisplayName:     displayName,
		AvatarURL:       info.AvatarURL,
	}, nil
}

func fetchGitHubPrimaryEmail(client *http.Client, accessToken string) (string, bool, error) {
	req, err := http.NewRequest(http.MethodGet, githubEmailsURL, nil)
	if err != nil {
		return "", false, fmt.Errorf("build github emails request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := client.Do(req)
	if err != nil {
		return "", false, fmt.Errorf("%w: %v", errOAuthUserInfoFailed, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", false, fmt.Errorf("read github emails response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", false, fmt.Errorf("%w: emails status %d", errOAuthUserInfoFailed, resp.StatusCode)
	}

	var emails []githubEmailInfo
	if err := json.Unmarshal(body, &emails); err != nil {
		return "", false, fmt.Errorf("parse github emails response: %w", err)
	}
	for _, email := range emails {
		normalized := strings.ToLower(strings.TrimSpace(email.Email))
		if email.Primary && normalized != "" {
			return normalized, email.Verified, nil
		}
	}
	return "", false, nil
}

func isRecordNotFound(err error) bool {
	return errors.Is(err, gorm.ErrRecordNotFound) || errors.Is(err, repository.ErrUserNotFound) || errors.Is(err, repository.ErrAccountIdentityNotFound)
}
