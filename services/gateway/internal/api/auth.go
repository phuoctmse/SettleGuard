package api

import (
	"context"
	"log"
	"net/http"
	"strings"

	"github.com/google/uuid"

	"github.com/phuoctmse/settleguard/gateway/internal/httperror"
)

// HeaderClientID is set by the gateway on every authenticated request and
// forwarded upstream. No backend service trusts it yet (spec §5); it exists
// so the contract is in place before ownership enforcement lands.
const HeaderClientID = "X-Client-Id"

// KeyLookup resolves a raw API key to the client_id it was issued to.
// Satisfied by *auth.Repository; kept as a one-method interface so this
// package does not import auth and tests can supply the real repository.
type KeyLookup interface {
	Lookup(ctx context.Context, raw string) (uuid.UUID, error)
}

type clientIDKey struct{}

// ClientIDFromContext returns the client_id APIKeyAuth stored for this
// request, if any.
func ClientIDFromContext(ctx context.Context) (uuid.UUID, bool) {
	id, ok := ctx.Value(clientIDKey{}).(uuid.UUID)
	return id, ok
}

// ContextWithClientID stores id the way APIKeyAuth does. Exported so tests
// of middleware that run after auth can build a request that already
// carries a client, without going through a database.
func ContextWithClientID(ctx context.Context, id uuid.UUID) context.Context {
	return context.WithValue(ctx, clientIDKey{}, id)
}

// unauthorizedMessage is deliberately the same for a missing, malformed,
// unknown or revoked key: the response must not reveal whether a key exists.
const unauthorizedMessage = "unauthorized"

// APIKeyAuth requires Authorization: Bearer <key>. On success it stores the
// client_id in the context and SETS X-Client-Id on the request -- always
// Set, never Add, so any value the caller supplied is discarded.
//
// The name is a fixed cross-task contract (plan Task 5 "Produces", consumed
// verbatim by Task 8 as api.APIKeyAuth); do not rename to KeyAuth.
//
//nolint:revive
func APIKeyAuth(keys KeyLookup) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			raw, ok := bearerToken(r.Header.Get("Authorization"))
			if !ok {
				httperror.Write(w, http.StatusUnauthorized, unauthorizedMessage)
				return
			}

			clientID, err := keys.Lookup(r.Context(), raw)
			if err != nil {
				// Distinct reasons are logged (with the request id) but not returned.
				log.Printf("gateway: auth failed request_id=%s: %v", r.Header.Get(HeaderRequestID), err)
				httperror.Write(w, http.StatusUnauthorized, unauthorizedMessage)
				return
			}

			// The gateway is the trust boundary. Everything upstream sees is
			// the gateway's assertion, never the client's:
			//   - Authorization: the key is verified here and goes no further --
			//     no backend reads it, and forwarding a credential to four
			//     processes that have no use for it is a leak waiting to happen.
			//   - Connection: ReverseProxy strips every header the client names
			//     in Connection AFTER the director runs, so a client sending
			//     "Connection: X-Client-Id" would erase the header set below.
			//     The gateway terminates the client's connection; the client's
			//     hop-by-hop declarations have no business reaching upstream.
			//   - X-Client-Id: Set, never Add, so any client-supplied value is
			//     replaced rather than merely preceded.
			r.Header.Del("Authorization")
			r.Header.Del("Connection")
			r.Header.Set(HeaderClientID, clientID.String())
			ctx := ContextWithClientID(r.Context(), clientID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// bearerToken extracts the token from "Bearer <token>", case-insensitively
// on the scheme, rejecting anything else.
func bearerToken(header string) (string, bool) {
	const scheme = "bearer "
	if len(header) <= len(scheme) || !strings.EqualFold(header[:len(scheme)], scheme) {
		return "", false
	}
	token := strings.TrimSpace(header[len(scheme):])
	return token, token != ""
}
