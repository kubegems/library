package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/crypto/ssh"
	"kubegems.io/library/rest/response"
)

const AnonymousUser = "anonymous" // anonymous username

type OIDCOptions struct {
	Issuer   string `json:"issuer" description:"oidc issuer url"`
	Insecure bool   `json:"insecure" description:"skip issuer and audience verification (optional)"`
	Audience string `json:"audience" description:"oidc resource server audience (optional)"`
}

type UserInfo struct {
	ID            string              `json:"id,omitempty"`
	Name          string              `json:"name,omitempty"`
	Email         string              `json:"email,omitempty"`
	EmailVerified bool                `json:"email_verified,omitempty"`
	Groups        []string            `json:"groups,omitempty"`
	Extra         map[string][]string `json:"extra,omitempty"`
}

type AuthenticateInfo struct {
	// Audiences is the set of audiences the authenticator was able to validate
	// the token against. If the authenticator is not audience aware, this field
	// will be empty.
	Audiences []string
	// User is the UserInfo associated with the authentication context.
	User UserInfo
}

type Decision int

const (
	DecisionDeny Decision = iota
	DecisionAllow
	DecisionNoOpinion
)

type TokenAuthenticator interface {
	// Authenticate authenticates the token and returns the authentication info.
	// if can't authenticate, return nil, "reason message", nil
	// if unexpected error, return nil, "", err
	Authenticate(ctx context.Context, token string) (*AuthenticateInfo, error)
}

type UsernamePasswordAuthenticator interface {
	Authenticate(ctx context.Context, username, password string) (*AuthenticateInfo, error)
}

type HTTPAuthenticator interface {
	Authenticate(ctx context.Context, r *http.Request) (*AuthenticateInfo, error)
}

type SSHAuthenticator interface {
	UsernamePasswordAuthenticator
	AuthenticatePublibcKey(ctx context.Context, pubkey ssh.PublicKey) (*AuthenticateInfo, error)
}

func NewBasicAuthenticationFilter(authenticator UsernamePasswordAuthenticator) Filter {
	return NewAuthenticateFilter(func(w http.ResponseWriter, r *http.Request) (*AuthenticateInfo, error) {
		username, password, ok := r.BasicAuth()
		if !ok {
			return nil, fmt.Errorf("no basic auth")
		}
		return authenticator.Authenticate(r.Context(), username, password)
	})
}

func NewTokenAuthenticationFilter(authenticator TokenAuthenticator) Filter {
	return NewAuthenticateFilter(func(w http.ResponseWriter, r *http.Request) (*AuthenticateInfo, error) {
		token := ExtracTokenFromRequest(r)
		if token == "" {
			// https://developer.mozilla.org/zh-CN/docs/Web/HTTP/Headers/WWW-Authenticate
			w.Header().Set("WWW-Authenticate", "Bearer")
			return nil, fmt.Errorf("no token found")
		}
		return authenticator.Authenticate(r.Context(), token)
	})
}

func NewHTTPAuthenticationFilter(authenticator HTTPAuthenticator) Filter {
	return NewAuthenticateFilter(func(w http.ResponseWriter, r *http.Request) (*AuthenticateInfo, error) {
		return authenticator.Authenticate(r.Context(), r)
	})
}

func NewAuthenticateFilter(on func(w http.ResponseWriter, r *http.Request) (*AuthenticateInfo, error)) Filter {
	return FilterFunc(func(w http.ResponseWriter, r *http.Request, next http.Handler) {
		info, err := on(w, r)
		if err != nil {
			response.Unauthorized(w, fmt.Sprintf("Unauthorized: %v", err))
			return
		}
		ctx := WithAuthenticate(r.Context(), *info)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func ExtracTokenFromRequest(r *http.Request) string {
	token := r.Header.Get("Authorization")
	if token == "" {
		token = r.URL.Query().Get("token")
	}
	return token
}

type authenticateContext struct{}

var authenticateContextKey = &authenticateContext{}

func WithAuthenticate(ctx context.Context, info AuthenticateInfo) context.Context {
	return context.WithValue(ctx, authenticateContextKey, info)
}

func AuthenticateFromContext(ctx context.Context) AuthenticateInfo {
	if info, ok := ctx.Value(authenticateContextKey).(AuthenticateInfo); ok {
		return info
	}
	return AuthenticateInfo{}
}

type AuthenticatorChain []TokenAuthenticator

var _ TokenAuthenticator = AuthenticatorChain{}

func (c AuthenticatorChain) Authenticate(ctx context.Context, token string) (*AuthenticateInfo, error) {
	var lasterr error
	for _, authn := range c {
		info, err := authn.Authenticate(ctx, token)
		if err != nil {
			lasterr = err
			continue
		}
		return info, nil
	}
	return nil, lasterr
}

type OIDCAuthenticator struct {
	Verifier               *oidc.IDTokenVerifier
	UsernameClaimCandidate []string
	EmailClaimCandidate    []string
	GroupsClaimCandidate   []string
	EmailToUsername        func(email string) string
}

var _ TokenAuthenticator = &OIDCAuthenticator{}

func NewOIDCAuthenticator(ctx context.Context, opts *OIDCOptions) (*OIDCAuthenticator, error) {
	// no oidc
	if opts.Issuer == "" {
		return nil, fmt.Errorf("oidc issuer is required")
	}
	ctx = oidc.InsecureIssuerURLContext(ctx, opts.Issuer)
	provider, err := oidc.NewProvider(ctx, opts.Issuer)
	if err != nil {
		return nil, fmt.Errorf("init oidc provider: %v", err)
	}
	verifier := provider.Verifier(&oidc.Config{
		SkipClientIDCheck: opts.Audience == "",
		SkipIssuerCheck:   true,
	})
	return &OIDCAuthenticator{
		Verifier:               verifier,
		UsernameClaimCandidate: []string{"preferred_username", "name", "email"},
		EmailClaimCandidate:    []string{"email"},
		GroupsClaimCandidate:   []string{"groups", "roles"},
		EmailToUsername: func(email string) string {
			return strings.Split(email, "@")[0]
		},
	}, nil
}

func (o *OIDCAuthenticator) Authenticate(ctx context.Context, token string) (*AuthenticateInfo, error) {
	if token == "" {
		return nil, fmt.Errorf("no token found")
	}
	token = strings.TrimPrefix(token, "Bearer ")
	idToken, err := o.Verifier.Verify(ctx, token)
	if err != nil {
		return nil, fmt.Errorf("oidc: verify token: %v", err)
	}
	var c claims
	if err := idToken.Claims(&c); err != nil {
		return nil, fmt.Errorf("oidc: parse claims: %v", err)
	}
	// username
	var username string
	for _, candidate := range o.UsernameClaimCandidate {
		if err := c.unmarshalClaim(candidate, &username); err != nil {
			continue
		}
		if username != "" {
			break
		}
	}
	// email
	var email string
	for _, candidate := range o.EmailClaimCandidate {
		if err := c.unmarshalClaim(candidate, &email); err != nil {
			continue
		}
		// If the email_verified claim is present, ensure the email is valid.
		// https://openid.net/specs/openid-connect-core-1_0.html#StandardClaims
		if hasEmailVerified := c.hasClaim("email_verified"); hasEmailVerified {
			var emailVerified bool
			if err := c.unmarshalClaim("email_verified", &emailVerified); err != nil {
				return nil, fmt.Errorf("oidc: parse 'email_verified' claim: %v", err)
			}
			// If the email_verified claim is present we have to verify it is set to `true`.
			if !emailVerified {
				return nil, fmt.Errorf("oidc: email not verified")
			}
		}
		if email != "" {
			break
		}
	}
	// if no username, use email as username
	if username == "" {
		if email != "" {
			username = o.EmailToUsername(email)
		} else {
			return nil, fmt.Errorf("oidc: no username/email claim found")
		}
	}
	// groups
	var groups stringOrArray
	for _, candidate := range o.GroupsClaimCandidate {
		if c.hasClaim(candidate) {
			if err := c.unmarshalClaim(candidate, &groups); err != nil {
				return nil, fmt.Errorf("oidc: parse groups claim %q: %v", candidate, err)
			}
			break
		}
	}
	info := UserInfo{
		ID:     idToken.Subject,
		Name:   username,
		Email:  email,
		Groups: groups,
	}
	return &AuthenticateInfo{Audiences: idToken.Audience, User: info}, nil
}

type claims map[string]json.RawMessage

func (c claims) unmarshalClaim(name string, v interface{}) error {
	val, ok := c[name]
	if !ok {
		return fmt.Errorf("claim not present")
	}
	return json.Unmarshal([]byte(val), v)
}

func (c claims) hasClaim(name string) bool {
	if _, ok := c[name]; !ok {
		return false
	}
	return true
}

type stringOrArray []string

func (s *stringOrArray) UnmarshalJSON(b []byte) error {
	var a []string
	if err := json.Unmarshal(b, &a); err == nil {
		*s = a
		return nil
	}
	var str string
	if err := json.Unmarshal(b, &str); err != nil {
		return err
	}
	*s = []string{str}
	return nil
}

func NewAnonymousAuthenticator() *AnonymousAuthenticator {
	return &AnonymousAuthenticator{}
}

type AnonymousAuthenticator struct{}

var _ TokenAuthenticator = &AnonymousAuthenticator{}

func (a *AnonymousAuthenticator) Authenticate(ctx context.Context, token string) (*AuthenticateInfo, error) {
	return &AuthenticateInfo{User: UserInfo{Name: AnonymousUser, Groups: []string{}}}, nil
}

var _ TokenAuthenticator = &LRUCacheAuthenticator{}

func NewCacheAuthenticator(authenticator TokenAuthenticator, size int, ttl time.Duration) *LRUCacheAuthenticator {
	return &LRUCacheAuthenticator{
		Authenticator: authenticator,
		Cache:         NewLRUCache[*AuthenticateInfo](size, ttl),
	}
}

type LRUCacheAuthenticator struct {
	Authenticator TokenAuthenticator
	Cache         LRUCache[*AuthenticateInfo]
}

// Authenticate implements TokenAuthenticator.
func (a *LRUCacheAuthenticator) Authenticate(ctx context.Context, token string) (*AuthenticateInfo, error) {
	return a.Cache.GetOrAdd(token, func() (*AuthenticateInfo, error) {
		return a.Authenticator.Authenticate(ctx, token)
	})
}

func NewCachedSSHAuthenticator(authenticator SSHAuthenticator, size int, ttl time.Duration) *LRUCacheSSHAuthenticator {
	return &LRUCacheSSHAuthenticator{Authenticator: authenticator, Cache: NewLRUCache[*AuthenticateInfo](size, ttl)}
}

var _ SSHAuthenticator = &LRUCacheSSHAuthenticator{}

type LRUCacheSSHAuthenticator struct {
	Authenticator SSHAuthenticator
	Cache         LRUCache[*AuthenticateInfo]
}

// AuthenticatePublibcKey implements SSHAuthenticator.
func (a *LRUCacheSSHAuthenticator) AuthenticatePublibcKey(ctx context.Context, pubkey ssh.PublicKey) (*AuthenticateInfo, error) {
	return a.Cache.GetOrAdd(ssh.FingerprintSHA256(pubkey), func() (*AuthenticateInfo, error) {
		return a.Authenticator.AuthenticatePublibcKey(ctx, pubkey)
	},
	)
}

// AuthenticatePassword implements SSHAuthenticator.
func (a *LRUCacheSSHAuthenticator) Authenticate(ctx context.Context, username, password string) (*AuthenticateInfo, error) {
	return a.Cache.GetOrAdd(fmt.Sprintf("%s:%s", username, password), func() (*AuthenticateInfo, error) {
		return a.Authenticator.Authenticate(ctx, username, password)
	})
}
