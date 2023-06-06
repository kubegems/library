package httpproxy

import (
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
)

type Server struct {
	Prefix string // prefix path
}

func (h *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	rp := httputil.ReverseProxy{
		Director: func(r *http.Request) {
			if forwardedHost := r.Header.Get("X-Forwarded-Host"); forwardedHost != "" {
				r.Host, r.URL.Host = forwardedHost, forwardedHost
			}
			if forwardedFor := r.Header.Get("X-Forwarded-For"); forwardedFor != "" {
				r.RemoteAddr = forwardedFor
			}
			if forwardedScheme := r.Header.Get("X-Forwarded-Scheme"); forwardedScheme != "" {
				r.URL.Scheme = forwardedScheme
			}
			if r.URL.Scheme == "" {
				r.URL.Scheme = "http"
			}
			if forwardedUri := getHeader(r, "X-Uri", "X-Forwarded-Uri"); forwardedUri != "" {
				if uri, err := url.ParseRequestURI(forwardedUri); err == nil {
					if uri.Path != "" {
						r.URL.Path, r.URL.RawPath = uri.Path, uri.RawPath
					}
					if uri.RawQuery != "" {
						r.URL.RawQuery = uri.RawQuery
					}
				}
			} else {
				// fallback to r.URL.Path
				r.URL.Path, r.URL.RawPath = strings.TrimPrefix(r.URL.Path, h.Prefix), ""
			}
			r.RequestURI = ""
		},
	}
	rp.ServeHTTP(w, r)
}

type Client struct {
	Server     *url.URL // server address
	HttpClient *http.Client
}

func (h Client) RoundTrip(r *http.Request) (*http.Response, error) {
	if r.URL.Host != "" {
		// SetXForwarded use r.Host to set X-Forwarded-Host,make sure r.Host is not empty
		r.Host = r.URL.Host
	}
	ctx := r.Context()
	outreq := r.Clone(ctx)
	if r.ContentLength == 0 {
		outreq.Body = nil
	}
	if outreq.Body != nil {
		defer outreq.Body.Close()
	}
	if outreq.Header == nil {
		outreq.Header = make(http.Header)
	}
	// Request.RequestURI can't be set in client requests
	outreq.RequestURI = ""
	pr := &httputil.ProxyRequest{In: r, Out: outreq}
	SetXForwarded(pr)
	pr.SetURL(h.Server)

	httpcli := http.DefaultClient
	if h.HttpClient != nil {
		httpcli = h.HttpClient
	}
	return httpcli.Do(outreq)
}

// SetXForwarded is a extend version of ProxyRequest{}.SetXForwarded(
func SetXForwarded(r *httputil.ProxyRequest) {
	if clientIP, _, err := net.SplitHostPort(r.In.RemoteAddr); err == nil {
		if prior := r.Out.Header["X-Forwarded-For"]; len(prior) > 0 {
			clientIP = strings.Join(prior, ", ") + ", " + clientIP
		}
		r.Out.Header.Set("X-Forwarded-For", clientIP)
	}

	if r.In.Header.Get("X-Forwarded-Host") == "" {
		if host := r.In.Host; host != "" {
			r.Out.Header.Set("X-Forwarded-Host", host)
		}
	}

	if r.In.Header.Get("X-Forwarded-Proto") == "" {
		if r.In.TLS == nil {
			r.Out.Header.Set("X-Forwarded-Proto", "http")
		} else {
			r.Out.Header.Set("X-Forwarded-Proto", "https")
		}
	}

	if r.In.Header.Get("X-Forwarded-Uri") == "" {
		if uri := r.In.URL.RequestURI(); uri != "" {
			r.Out.Header.Set("X-Forwarded-Uri", uri)
			// k8s apiserver proxy overwrites "X-Forwarded-Uri"
			// we use "X-Uri" as a workaround
			r.Out.Header.Set("X-Uri", uri)
		}
	}

	if r.In.Header.Get("X-Forwarded-Scheme") == "" {
		if scheme := r.In.URL.Scheme; scheme != "" {
			r.Out.Header.Set("X-Forwarded-Scheme", scheme)
		}
	}
}

func getHeader(r *http.Request, keys ...string) string {
	for _, key := range keys {
		if v := r.Header.Get(key); v != "" {
			return v
		}
	}
	return ""
}
