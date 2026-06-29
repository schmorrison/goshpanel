package auth

import "net/http"

// CSRFProtect validates CSRF tokens on mutating requests.
func (s *Service) CSRFProtect(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
			if err := r.ParseForm(); err != nil {
				http.Error(w, "bad request", http.StatusBadRequest)
				return
			}

			token := r.FormValue("csrf_token")
			if token == "" {
				token = r.Header.Get("X-CSRF-Token")
			}
			if err := s.ValidateCSRF(r.Context(), token); err != nil {
				http.Error(w, "forbidden", http.StatusForbidden)
				return
			}
		}

		next.ServeHTTP(w, r)
	})
}
