package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

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
	Authenticate(ctx context.Context, token string) (*AuthenticateInfo, bool, error)
}

type SSHAuthenticator interface {
	AuthenticatePublibcKey(ctx context.Context, pubkey ssh.PublicKey) (*AuthenticateInfo, bool, error)
	AuthenticatePassword(ctx context.Context, username, password string) (*AuthenticateInfo, bool, error)
}

type AuthenticatorChain []TokenAuthenticator

var _ TokenAuthenticator = AuthenticatorChain{}

func (c AuthenticatorChain) Authenticate(ctx context.Context, token string) (*AuthenticateInfo, bool, error) {
	for _, authn := range c {
		info, ok, err := authn.Authenticate(ctx, token)
		if err != nil {
			return nil, false, err
		}
		if ok {
			return info, true, nil
		}
	}
	return nil, false, nil
}

func NewAuthenticationFilter(authenticator TokenAuthenticator) Filter {
	return FilterFunc(func(w http.ResponseWriter, r *http.Request, next http.Handler) {
		token := ExtracTokenFromRequest(r)
		info, ok, err := authenticator.Authenticate(r.Context(), token)
		if err != nil {
			response.Unauthorized(w, fmt.Sprintf("Unauthorized: %v", err))
			return
		}
		if ok {
			ctx := WithAuthenticate(r.Context(), *info)
			next.ServeHTTP(w, r.WithContext(ctx))
			return
		}
		response.Unauthorized(w, "Unauthorized")
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

type OIDCAuthenticator struct {
	Verifier               *oidc.IDTokenVerifier
	UsernameClaimCandidate []string
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
		UsernameClaimCandidate: []string{"email", "preferred_username", "name"},
		GroupsClaimCandidate:   []string{"groups", "roles"},
		EmailToUsername: func(email string) string {
			return strings.Split(email, "@")[0]
		},
	}, nil
}

func (o *OIDCAuthenticator) Authenticate(ctx context.Context, token string) (*AuthenticateInfo, bool, error) {
	if token == "" {
		return nil, false, nil
	}
	token = strings.TrimPrefix(token, "Bearer ")
	idToken, err := o.Verifier.Verify(ctx, token)
	if err != nil {
		return nil, false, fmt.Errorf("oidc: verify token: %v", err)
	}
	var c claims
	if err := idToken.Claims(&c); err != nil {
		return nil, false, fmt.Errorf("oidc: parse claims: %v", err)
	}
	// username
	var username string
	for _, candidate := range o.UsernameClaimCandidate {
		if err := c.unmarshalClaim(candidate, &username); err != nil {
			continue
		}
		// If the email_verified claim is present, ensure the email is valid.
		// https://openid.net/specs/openid-connect-core-1_0.html#StandardClaims
		if candidate == "email" {
			if hasEmailVerified := c.hasClaim("email_verified"); hasEmailVerified {
				var emailVerified bool
				if err := c.unmarshalClaim("email_verified", &emailVerified); err != nil {
					return nil, false, fmt.Errorf("oidc: parse 'email_verified' claim: %v", err)
				}
				// If the email_verified claim is present we have to verify it is set to `true`.
				if !emailVerified {
					return nil, false, fmt.Errorf("oidc: email not verified")
				}
			}
			username = o.EmailToUsername(username)
		}
		if username != "" {
			break
		}
	}
	if username == "" {
		return nil, false, fmt.Errorf("oidc: no username claim found")
	}
	// groups
	var groups stringOrArray
	for _, candidate := range o.GroupsClaimCandidate {
		if c.hasClaim(candidate) {
			if err := c.unmarshalClaim(candidate, &groups); err != nil {
				return nil, false, fmt.Errorf("oidc: parse groups claim %q: %v", candidate, err)
			}
			break
		}
	}
	info := UserInfo{
		ID:     idToken.Subject,
		Name:   username,
		Groups: groups,
	}
	return &AuthenticateInfo{Audiences: idToken.Audience, User: info}, true, nil
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

func (a *AnonymousAuthenticator) Authenticate(ctx context.Context, token string) (*AuthenticateInfo, bool, error) {
	return &AuthenticateInfo{User: UserInfo{Name: AnonymousUser, Groups: []string{}}}, true, nil
}
