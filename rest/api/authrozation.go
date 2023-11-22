package api

import (
	"context"
	"net/http"

	"kubegems.io/library/rest/response"
)

type Authorizer interface {
	Authorize(ctx context.Context, user UserInfo, a Attributes) (authorized Decision, reason string, err error)
}

type AuthorizerFunc func(ctx context.Context, user UserInfo, a Attributes) (authorized Decision, reason string, err error)

func (f AuthorizerFunc) Authorize(ctx context.Context, user UserInfo, a Attributes) (authorized Decision, reason string, err error) {
	return f(ctx, user, a)
}

func NewAlwaysAllowAuthorizer() Authorizer {
	return AuthorizerFunc(func(ctx context.Context, user UserInfo, a Attributes) (authorized Decision, reason string, err error) {
		return DecisionAllow, "", nil
	})
}

type AuthorizerChain []Authorizer

func (c AuthorizerChain) Authorize(ctx context.Context, user UserInfo, a Attributes) (Decision, string, error) {
	for _, authorizer := range c {
		decision, reason, err := authorizer.Authorize(ctx, user, a)
		if err != nil {
			return DecisionDeny, reason, err
		}
		if decision == DecisionAllow {
			return DecisionAllow, reason, nil
		}
		if decision == DecisionDeny {
			return DecisionDeny, reason, nil
		}
	}
	return DecisionDeny, "no decision", nil
}

func NewAuthorizationFilter(authorizer Authorizer) Filter {
	return FilterFunc(func(w http.ResponseWriter, r *http.Request, next http.Handler) {
		attributes := AttributesFromContext(r.Context())
		user := AuthenticateFromContext(r.Context()).User
		decision, reason, err := authorizer.Authorize(r.Context(), user, *attributes)
		if err != nil {
			response.InternalServerError(w, err)
			return
		}
		if decision == DecisionAllow {
			next.ServeHTTP(w, r)
			return
		}
		if decision == DecisionDeny {
			response.Forbidden(w, reason)
			return
		}
		// DecisionNoOpinion
		response.Forbidden(w, "access denied")
	})
}
