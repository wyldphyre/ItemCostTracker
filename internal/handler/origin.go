package handler

import (
	"log"
	"net/http"
	"net/url"
)

// safeMethods never mutate state, so they need no origin check.
var safeMethods = map[string]bool{
	http.MethodGet:     true,
	http.MethodHead:    true,
	http.MethodOptions: true,
}

// CheckOrigin rejects state-changing requests that a browser tells us came from
// another site. Without it, any page the user visits could silently POST to
// /import with mode=replace and wipe the store.
//
// Two signals are used, in order of reliability:
//
//   - Sec-Fetch-Site, sent by every current browser on every request. Only
//     "same-origin" and "none" (a direct navigation) are accepted.
//   - Origin, sent by browsers on cross-origin form submissions and on all
//     fetch/XHR requests, compared against the Host we were reached on.
//
// A request carrying neither header is not from a browser (curl, scripts, the
// backup tooling) and is allowed through — this guards against cross-site
// requests, not against a client that can set its own headers anyway.
func CheckOrigin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if safeMethods[r.Method] {
			next.ServeHTTP(w, r)
			return
		}

		if site := r.Header.Get("Sec-Fetch-Site"); site != "" {
			if site != "same-origin" && site != "none" {
				reject(w, r, "Sec-Fetch-Site: "+site)
				return
			}
			next.ServeHTTP(w, r)
			return
		}

		if origin := r.Header.Get("Origin"); origin != "" {
			u, err := url.Parse(origin)
			if err != nil || u.Host != r.Host {
				reject(w, r, "Origin: "+origin)
				return
			}
		}

		next.ServeHTTP(w, r)
	})
}

func reject(w http.ResponseWriter, r *http.Request, detail string) {
	log.Printf("rejected cross-site %s %s (%s)", r.Method, r.URL.Path, detail)
	http.Error(w, "cross-site request rejected", http.StatusForbidden)
}
