package httpapi

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	cookieName      = "bwd_session"
	loginTTL        = 30 * 24 * time.Hour
	maxLoginFails   = 10
	loginFailWindow = time.Minute
)

// auth authenticates the single admin account. The login cookie is a
// signed expiry time, so the server keeps no session table.
type auth struct {
	username, password string
	secret             []byte
	now                func() time.Time

	mu    sync.Mutex
	fails []time.Time
}

func (a *auth) checkCredentials(username, password string) bool {
	// Hash first so the comparison time does not depend on input length.
	u, wantU := sha256.Sum256([]byte(username)), sha256.Sum256([]byte(a.username))
	p, wantP := sha256.Sum256([]byte(password)), sha256.Sum256([]byte(a.password))
	okU := subtle.ConstantTimeCompare(u[:], wantU[:])
	okP := subtle.ConstantTimeCompare(p[:], wantP[:])
	return okU&okP == 1
}

// throttled reports whether too many logins failed recently.
func (a *auth) throttled() bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	cutoff := a.now().Add(-loginFailWindow)
	kept := a.fails[:0]
	for _, t := range a.fails {
		if t.After(cutoff) {
			kept = append(kept, t)
		}
	}
	a.fails = kept
	return len(a.fails) >= maxLoginFails
}

func (a *auth) recordFailure() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.fails = append(a.fails, a.now())
}

func (a *auth) sign(expiry string) string {
	mac := hmac.New(sha256.New, a.secret)
	mac.Write([]byte(expiry))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func (a *auth) newToken() string {
	expiry := strconv.FormatInt(a.now().Add(loginTTL).Unix(), 10)
	return expiry + "." + a.sign(expiry)
}

func (a *auth) validToken(token string) bool {
	expiry, sig, ok := strings.Cut(token, ".")
	if !ok || !hmac.Equal([]byte(sig), []byte(a.sign(expiry))) {
		return false
	}
	unix, err := strconv.ParseInt(expiry, 10, 64)
	return err == nil && a.now().Unix() < unix
}

func (a *auth) authenticated(r *http.Request) bool {
	c, err := r.Cookie(cookieName)
	return err == nil && a.validToken(c.Value)
}

func (a *auth) setCookie(w http.ResponseWriter, r *http.Request, value string, maxAge int) {
	http.SetCookie(w, &http.Cookie{
		Name:     cookieName,
		Value:    value,
		Path:     "/",
		MaxAge:   maxAge,
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
		Secure:   r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https",
	})
}
