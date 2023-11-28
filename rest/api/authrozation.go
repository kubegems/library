package api

import (
	"context"
	"net"
	"net/http"
	"time"

	"github.com/hashicorp/golang-lru/v2/expirable"
	"kubegems.io/library/rest/response"
)

type RequestAuthorizer interface {
	AuthorizeRequest(r *http.Request) (Decision, string, error)
}

type RequestAuthorizerFunc func(r *http.Request) (Decision, string, error)

func (f RequestAuthorizerFunc) AuthorizeRequest(r *http.Request) (Decision, string, error) {
	return f(r)
}

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

func NewAlwaysDenyAuthorizer() Authorizer {
	return AuthorizerFunc(func(ctx context.Context, user UserInfo, a Attributes) (authorized Decision, reason string, err error) {
		return DecisionDeny, "", nil
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

type authorizationContext struct{}

var authorizationContextKey = &authorizationContext{}

func WithAuthorizationContext(ctx context.Context, decision Decision) context.Context {
	return context.WithValue(ctx, authorizationContextKey, decision)
}

func AuthorizationContextFromContext(ctx context.Context) (Decision, bool) {
	decision, ok := ctx.Value(authorizationContextKey).(Decision)
	return decision, ok
}

func NewRequestAuthorizationFilter(on RequestAuthorizerFunc) Filter {
	return FilterFunc(func(w http.ResponseWriter, r *http.Request, next http.Handler) {
		// already authorized by previous filter
		if decision, ok := AuthorizationContextFromContext(r.Context()); ok {
			if decision == DecisionAllow {
				next.ServeHTTP(w, r)
				return
			}
			if decision == DecisionDeny {
				response.Forbidden(w, "access denied")
				return
			}
		}
		decision, reason, err := on(r)
		if err != nil {
			response.InternalServerError(w, err)
			return
		}
		if decision == DecisionAllow {
			// allow next filter to skip authorization
			r = r.WithContext(WithAuthorizationContext(r.Context(), decision))
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

func NewAllowCIDRAuthorizer(cidrs []string, defaultDec Decision) RequestAuthorizer {
	return RequestAuthorizerFunc(func(r *http.Request) (Decision, string, error) {
		if RequestSourceIPInCIDR(cidrs, r) {
			return DecisionAllow, "", nil
		}
		return defaultDec, "", nil
	})
}

func RequestSourceIPInCIDR(cidrs []string, r *http.Request) bool {
	ip, _, _ := net.SplitHostPort(r.RemoteAddr)
	return InCIDR(ip, cidrs)
}

func InCIDR(ip string, cidrs []string) bool {
	for _, cidr := range cidrs {
		if cidr == ip {
			return true
		}
		// check if ip is in cidr
		_, ipn, err := net.ParseCIDR(cidr)
		if err != nil {
			continue
		}
		if ipn.Contains(net.ParseIP(ip)) {
			return true
		}
	}
	return false
}

func NewAuthorizationFilter(authorizer Authorizer) Filter {
	return NewRequestAuthorizationFilter(func(r *http.Request) (Decision, string, error) {
		attributes := AttributesFromContext(r.Context())
		if attributes == nil {
			return DecisionDeny, "no attributes", nil
		}
		user := AuthenticateFromContext(r.Context()).User
		return authorizer.Authorize(r.Context(), user, *attributes)
	})
}

func NewCacheAuthorizer(authorizer Authorizer, ttl time.Duration, size int) Authorizer {
	return &LRUCacheAuthorizer{
		Authorizer: authorizer,
		cache:      expirable.NewLRU[string, Decision](size, nil, ttl),
	}
}

type LRUCacheAuthorizer struct {
	Authorizer Authorizer
	cache      *expirable.LRU[string, Decision]
}

// Authorize implements Authorizer.
func (c *LRUCacheAuthorizer) Authorize(ctx context.Context, user UserInfo, a Attributes) (authorized Decision, reason string, err error) {
	if c.cache == nil {
		return c.Authorizer.Authorize(ctx, user, a)
	}
	act, expr := a.ToWildcards()
	key := user.Name + "@" + expr + ":" + act
	if decision, ok := c.cache.Get(key); ok {
		return decision, "", nil
	}
	decision, reason, err := c.Authorizer.Authorize(ctx, user, a)
	if err != nil {
		return decision, reason, err
	}
	if decision == DecisionAllow {
		c.cache.Add(key, decision)
	}
	return decision, reason, nil
}
