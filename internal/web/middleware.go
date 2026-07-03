package web

import (
	"context"
	"crypto/subtle"
	"net/http"

	"github.com/schmorrison/goshpanel/internal/store"
)

type ctxKey int

const (
	ctxUser ctxKey = iota
	ctxSession
)

const sessionCookie = "goshpanel_session"

// requireAuth resolves the session cookie, enforces CSRF on mutating
// requests and injects user/session into the request context.
func (s *Server) requireAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie(sessionCookie)
		if err != nil {
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}
		user, sess, err := s.auth.Authenticate(cookie.Value)
		if err != nil {
			s.clearSessionCookie(w)
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}

		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			token := r.FormValue("csrf_token")
			if token == "" {
				token = r.Header.Get("X-CSRF-Token")
			}
			if subtle.ConstantTimeCompare([]byte(token), []byte(sess.CSRFToken)) != 1 {
				http.Error(w, "invalid CSRF token", http.StatusForbidden)
				return
			}
		}

		ctx := context.WithValue(r.Context(), ctxUser, user)
		ctx = context.WithValue(ctx, ctxSession, sess)
		next(w, r.WithContext(ctx))
	}
}

// withBlocker rejects requests from denied IPs before anything else runs.
func (s *Server) withBlocker(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if s.blocker.Blocked(r.RemoteAddr) {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// withSecurityHeaders sets a strict CSP and friends on every response.
func (s *Server) withSecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("Content-Security-Policy", "default-src 'self'; style-src 'self'; img-src 'self' data:")
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Referrer-Policy", "no-referrer")
		next.ServeHTTP(w, r)
	})
}

func (s *Server) setSessionCookie(w http.ResponseWriter, sess store.Session) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookie,
		Value:    sess.Token,
		Path:     "/",
		Expires:  sess.ExpiresAt,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
}

func (s *Server) clearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookie,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
}

// currentUser returns the authenticated user from the request context.
func currentUser(r *http.Request) store.User {
	u, _ := r.Context().Value(ctxUser).(store.User)
	return u
}

// currentSession returns the session from the request context.
func currentSession(r *http.Request) store.Session {
	s, _ := r.Context().Value(ctxSession).(store.Session)
	return s
}

// audit records an action by the current user, logging failures.
func (s *Server) audit(r *http.Request, action, detail string) {
	if err := s.store.AppendAudit(currentUser(r).Username, action, detail); err != nil {
		s.log.Error("audit append failed", "err", err)
	}
}
