package auth

import "net/http"

func RequireSession(service *Service) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if isPublic(r) {
				next.ServeHTTP(w, r)
				return
			}
			cookie, err := r.Cookie(SessionCookieName)
			if err != nil {
				unauthorized(w)
				return
			}
			user, found, err := service.Authenticate(r.Context(), cookie.Value)
			if err != nil || !found {
				unauthorized(w)
				return
			}
			next.ServeHTTP(w, r.WithContext(userContext(r.Context(), user)))
		})
	}
}

func isPublic(r *http.Request) bool {
	if r.URL.Path == "/" || r.URL.Path == "/api/v1/health" || r.URL.Path == "/api/v1/ready" {
		return true
	}
	if r.URL.Path == "/api/v1/auth/login" || r.URL.Path == "/api/v1/auth/logout" {
		return true
	}
	return r.Method == http.MethodPost && r.URL.Path == "/api/v1/notifications"
}

func unauthorized(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnauthorized)
	_, _ = w.Write([]byte(`{"success":false,"error":"unauthorized"}` + "\n"))
}
