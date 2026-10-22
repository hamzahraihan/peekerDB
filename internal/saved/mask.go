package saved

// Masking a conn string for display.
//
// A conn string carries a password. It is written to disk (encrypted on
// Windows, plain 0600 elsewhere) but it must never be *displayed*: not
// in the picker, not on stdout, not in session.json, not in the
// clipboard. Mask is the single place that decides what a conn string
// looks like without its password.

import (
	"net/url"
	"strings"
)

// MaskPlaceholder stands in for a password in a masked conn string. It
// marks the slot, so an edit can leave the password alone and keep the
// stored one.
const MaskPlaceholder = "xxxxx"

// Mask renders a conn string for display with the password replaced by
// MaskPlaceholder. It never returns the password: a string url.Parse
// cannot read — a raw MySQL DSN, a password with a space, a malformed
// percent escape — is scrubbed textually rather than returned as-is.
func Mask(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return raw
	}
	if u, err := url.Parse(raw); err == nil && u.Host != "" {
		if u.User != nil {
			if _, has := u.User.Password(); has {
				u.User = url.UserPassword(u.User.Username(), MaskPlaceholder)
			}
		}
		return redactPasswordParams(u.Redacted())
	}
	return redactPasswordParams(scrubUserinfo(raw))
}

// Password returns the password text of a conn string, and whether there
// was one. The text is the raw form as it appears in the string, so it
// can be spliced back into a masked conn string unchanged.
func Password(raw string) (string, bool) {
	raw = strings.TrimSpace(raw)
	start, end, ok := userinfoSpan(raw)
	if !ok {
		return "", false
	}
	userinfo := raw[start:end]
	c := strings.Index(userinfo, ":")
	if c < 0 {
		return "", false
	}
	return userinfo[c+1:], true
}

// MergeEditedConn returns the conn string to save after the user edited
// a masked one. When the password slot still holds the placeholder, the
// stored password is restored; otherwise the user typed a new password
// and their text wins.
func MergeEditedConn(entered, stored string) string {
	entered = strings.TrimSpace(entered)
	stored = strings.TrimSpace(stored)
	if entered == "" || stored == "" {
		return entered
	}
	start, end, ok := userinfoSpan(entered)
	if !ok {
		return entered
	}
	userinfo := entered[start:end]
	c := strings.Index(userinfo, ":")
	if c < 0 || userinfo[c+1:] != MaskPlaceholder {
		return entered // the user typed a password, or there is no slot
	}
	pass, ok := Password(stored)
	if !ok {
		return entered
	}
	return entered[:start+c+1] + pass + entered[end:]
}

// scrubUserinfo replaces the password of a conn string that url.Parse
// could not read. It works on the text: the password is the text after
// the first colon of the userinfo, up to the "@" that ends it.
func scrubUserinfo(raw string) string {
	start, end, ok := userinfoSpan(raw)
	if !ok {
		return raw
	}
	userinfo := raw[start:end]
	c := strings.Index(userinfo, ":")
	if c < 0 {
		return raw
	}
	return raw[:start+c+1] + MaskPlaceholder + raw[end:]
}

// userinfoSpan returns the bounds of the userinfo section of a conn
// string — the part between the scheme and the host, where the
// password lives. ok is false when the string carries no userinfo.
func userinfoSpan(raw string) (start, end int, ok bool) {
	if i := strings.Index(raw, "://"); i >= 0 {
		rest := raw[i+3:]
		// The authority ends at the first path, query or fragment.
		auth := rest
		if j := strings.IndexAny(rest, "/?#"); j >= 0 {
			auth = rest[:j]
		}
		// The last "@" of the authority separates the userinfo, so a
		// password may itself hold an "@" ("u:p@ss@h").
		at := strings.LastIndex(auth, "@")
		if at < 0 {
			return 0, 0, false
		}
		return i + 3, i + 3 + at, true
	}
	// The raw MySQL DSN form: user:pass@tcp(host:3306)/db. A path
	// ("./my:db@x") also has an "@", but a DSN userinfo never holds a
	// path separator.
	at := strings.Index(raw, "@")
	if at < 0 {
		return 0, 0, false
	}
	if strings.ContainsAny(raw[:at], "/\\") {
		return 0, 0, false
	}
	return 0, at, true
}

// redactPasswordParams blanks the value of every password-carrying query
// parameter. pgx and go-mssqldb both accept ?password=..., and the
// password would otherwise be printed in the clear.
func redactPasswordParams(s string) string {
	q := strings.Index(s, "?")
	if q < 0 {
		return s
	}
	head, query := s[:q+1], s[q+1:]
	frag := ""
	if i := strings.Index(query, "#"); i >= 0 {
		query, frag = query[:i], query[i:]
	}
	parts := strings.Split(query, "&")
	for i, p := range parts {
		key := p
		if j := strings.Index(p, "="); j >= 0 {
			key = p[:j]
		}
		if k, err := url.QueryUnescape(key); err == nil {
			key = k
		}
		if !isPasswordParam(key) {
			continue
		}
		if j := strings.Index(p, "="); j >= 0 {
			parts[i] = p[:j+1] + MaskPlaceholder
		} else {
			parts[i] = p + "=" + MaskPlaceholder
		}
	}
	return head + strings.Join(parts, "&") + frag
}

// isPasswordParam reports whether a query parameter carries a password.
func isPasswordParam(key string) bool {
	switch strings.ToLower(strings.TrimSpace(key)) {
	case "password", "passwd", "pwd", "sslpassword":
		return true
	}
	return false
}
