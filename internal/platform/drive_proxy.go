package platform

import (
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strings"
)

func (a *App) driveProxy() http.Handler {
	target, err := url.Parse(os.Getenv("DRIVE_INTERNAL_URL"))
	if err != nil || target.Host == "" || (target.Scheme != "http" && target.Scheme != "https") {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			writeError(w, 503, "Drive backend chưa cấu hình.", "SERVICE_UNAVAILABLE")
		})
	}
	proxy := httputil.NewSingleHostReverseProxy(target)
	director := proxy.Director
	proxy.Director = func(r *http.Request) {
		director(r)
		r.URL.Path = "/api/" + strings.TrimPrefix(r.URL.Path, "/api/drive/")
		r.URL.RawPath = ""
		r.Header.Set("X-Drive-Auth-Mode", "platform")
		r.Header.Del("X-Drive-Actor")
	}
	proxy.ModifyResponse = func(response *http.Response) error { response.Header.Del("Set-Cookie"); return nil }
	proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		writeError(w, 503, "Drive backend chưa sẵn sàng.", "SERVICE_UNAVAILABLE")
	}
	return a.auth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/drive/login" || r.URL.Path == "/api/drive/logout" {
			writeError(w, 403, "Dùng phiên đăng nhập desktop.", "ACCESS_DENIED")
			return
		}
		proxy.ServeHTTP(w, r)
	}))
}
