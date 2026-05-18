package constants

type ContextKey string

const (
	RequestIDCtx  ContextKey = "RequestID"
	AuthUserIDCtx ContextKey = "AuthUserID"
	AuthEmailCtx  ContextKey = "AuthEmail"
)
