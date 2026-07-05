package api

import (
	"encoding/json"
	"net/http"
)

type ErrorCode string

const (
	CodeInternalError           ErrorCode = "internal.error"
	CodeInvalidRequest          ErrorCode = "validation.invalid_request"
	CodeUnauthorized            ErrorCode = "auth.unauthorized"
	CodeAuthInvalidEmail        ErrorCode = "auth.invalid_email"
	CodeAuthInvalidCode         ErrorCode = "auth.invalid_code"
	CodeAuthSessionRequired     ErrorCode = "auth.session_required"
	CodeAuthSessionExpired      ErrorCode = "auth.session_expired"
	CodeAuthOAuthStateInvalid   ErrorCode = "auth.oauth_state_invalid"
	CodeAuthOAuthEmailTaken     ErrorCode = "auth.email_taken"
	CodeAuthIdentityBound       ErrorCode = "auth.identity_already_bound"
	CodeAuthTermsRequired       ErrorCode = "auth.terms_required"
	CodeAuthOAuthNotConfigured  ErrorCode = "auth.oauth_not_configured"
	CodeWorkspaceNotFound       ErrorCode = "workspace.not_found"
	CodeWorkspaceLimit          ErrorCode = "workspace.limit_exceeded"
	CodePermissionDenied        ErrorCode = "workspace.permission_denied"
	CodeAPIKeyNotFound          ErrorCode = "api_key.not_found"
	CodeAPIKeyInvalidState      ErrorCode = "api_key.invalid_state"
	CodeAPIKeyInvalidName       ErrorCode = "api_key.invalid_name"
	CodeAPIKeyInvalidLimit      ErrorCode = "api_key.invalid_limit"
	CodeAPIKeyInvalidExpiration ErrorCode = "api_key.invalid_expiration"
	CodeModelNotFound           ErrorCode = "model.not_found"
	CodeModelInvalidQuery       ErrorCode = "model.invalid_query"
	CodeBillingDuplicate        ErrorCode = "billing.duplicate_conflict"
	CodeInsufficientBalance     ErrorCode = "billing.insufficient_balance"
)

type AppError struct {
	Code       ErrorCode
	Message    string
	HTTPStatus int
}

var (
	ErrInternalError           = AppError{Code: CodeInternalError, Message: "Internal error", HTTPStatus: http.StatusInternalServerError}
	ErrInvalidRequest          = AppError{Code: CodeInvalidRequest, Message: "Invalid request", HTTPStatus: http.StatusBadRequest}
	ErrUnauthorized            = AppError{Code: CodeUnauthorized, Message: "Unauthorized", HTTPStatus: http.StatusUnauthorized}
	ErrInvalidEmail            = AppError{Code: CodeAuthInvalidEmail, Message: "Invalid email", HTTPStatus: http.StatusBadRequest}
	ErrInvalidCode             = AppError{Code: CodeAuthInvalidCode, Message: "Invalid code", HTTPStatus: http.StatusBadRequest}
	ErrSessionRequired         = AppError{Code: CodeAuthSessionRequired, Message: "Session required", HTTPStatus: http.StatusUnauthorized}
	ErrSessionExpired          = AppError{Code: CodeAuthSessionExpired, Message: "Session expired", HTTPStatus: http.StatusUnauthorized}
	ErrOAuthStateInvalid       = AppError{Code: CodeAuthOAuthStateInvalid, Message: "Invalid OAuth state", HTTPStatus: http.StatusBadRequest}
	ErrOAuthEmailTaken         = AppError{Code: CodeAuthOAuthEmailTaken, Message: "This email is already registered. Please sign in first and link Google in account settings.", HTTPStatus: http.StatusConflict}
	ErrOAuthIdentityBound      = AppError{Code: CodeAuthIdentityBound, Message: "This Google account is already linked to another user", HTTPStatus: http.StatusConflict}
	ErrTermsRequired           = AppError{Code: CodeAuthTermsRequired, Message: "Terms acceptance required", HTTPStatus: http.StatusForbidden}
	ErrOAuthNotConfigured      = AppError{Code: CodeAuthOAuthNotConfigured, Message: "OAuth provider is not configured", HTTPStatus: http.StatusServiceUnavailable}
	ErrWorkspaceNotFound       = AppError{Code: CodeWorkspaceNotFound, Message: "Workspace not found", HTTPStatus: http.StatusNotFound}
	ErrWorkspaceLimit          = AppError{Code: CodeWorkspaceLimit, Message: "Workspace limit exceeded", HTTPStatus: http.StatusConflict}
	ErrPermissionDenied        = AppError{Code: CodePermissionDenied, Message: "Permission denied", HTTPStatus: http.StatusForbidden}
	ErrAPIKeyNotFound          = AppError{Code: CodeAPIKeyNotFound, Message: "API key not found", HTTPStatus: http.StatusNotFound}
	ErrAPIKeyInvalidState      = AppError{Code: CodeAPIKeyInvalidState, Message: "API key invalid state", HTTPStatus: http.StatusConflict}
	ErrAPIKeyInvalidName       = AppError{Code: CodeAPIKeyInvalidName, Message: "Invalid API key name", HTTPStatus: http.StatusBadRequest}
	ErrAPIKeyInvalidLimit      = AppError{Code: CodeAPIKeyInvalidLimit, Message: "Invalid API key limit", HTTPStatus: http.StatusBadRequest}
	ErrAPIKeyInvalidExpiration = AppError{Code: CodeAPIKeyInvalidExpiration, Message: "Invalid API key expiration", HTTPStatus: http.StatusBadRequest}
	ErrModelNotFound           = AppError{Code: CodeModelNotFound, Message: "Model not found", HTTPStatus: http.StatusNotFound}
	ErrModelInvalidQuery       = AppError{Code: CodeModelInvalidQuery, Message: "Invalid model query", HTTPStatus: http.StatusBadRequest}
	ErrBillingDuplicate        = AppError{Code: CodeBillingDuplicate, Message: "Duplicate billing event conflict", HTTPStatus: http.StatusConflict}
	ErrInsufficientBalance     = AppError{Code: CodeInsufficientBalance, Message: "Insufficient balance", HTTPStatus: http.StatusPaymentRequired}
)

type errorResponse struct {
	Error errorBody `json:"error"`
}

type errorBody struct {
	Code      ErrorCode `json:"code"`
	Message   string    `json:"message"`
	RequestID string    `json:"request_id"`
}

func WriteError(w http.ResponseWriter, requestID string, appErr AppError) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(appErr.HTTPStatus)
	_ = json.NewEncoder(w).Encode(errorResponse{
		Error: errorBody{
			Code:      appErr.Code,
			Message:   appErr.Message,
			RequestID: requestID,
		},
	})
}
