package gateway

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"konsulin-service/internal/pkg/utils"

	"github.com/konsulin-care/gateway-sdk-go/verify"
	"github.com/konsulin-care/gateway-sdk-go/wire"
	"github.com/redis/go-redis/v9"
)

// jwksPath is the gateway JWKS endpoint relative to APP_GATEWAY_URL: the
// verifier fetches APP_GATEWAY_URL/.well-known/jwks for inbound tokens.
const jwksPath = "/.well-known/jwks"

// VerifyMiddleware builds the inbound gateway-callback middleware for the
// given token kinds (ks-event+jwt, ks-delivery+jwt): it wraps the SDK
// verify middleware over a verifier whose gateway JWKS is
// APP_GATEWAY_URL/.well-known/jwks, whose aud is this party's own id
// (APP_PARTY_ID), and whose webhook_id replay cache is the Redis-backed
// ReplayCache (T06) over REDIS_HOST/REDIS_PORT/REDIS_PASSWORD. One SDK
// middleware is built per kind; the wrapper dispatches each request on the
// bearer token's typ header, so only a token of an allowed kind reaches its
// verifier. It fails closed at construction (no kinds, empty
// APP_GATEWAY_URL or APP_PARTY_ID, or a kind that is not a gateway-signed
// typ) and per request (401 without valid bearer credentials, without a
// decodable typ, or with a typ outside kinds) before the next handler runs.
// A duplicate Webhook-Id is acknowledged 200 by the SDK middleware without
// calling next; verified claims are readable via ClaimsFromContext.
//
// Parameters: kinds are the expected token typ values (e.g.
// wire.TypEvent, wire.TypDelivery); at least one is required.
// Returns: a func(http.Handler) http.Handler that wraps the callback
// handler, and nil, or an error naming only configuration facts. Never
// panics; never logs key values.
func VerifyMiddleware(kinds ...string) (func(http.Handler) http.Handler, error) {
	byKind, err := buildKindMiddlewares(kinds)
	if err != nil {
		return nil, err
	}
	return func(next http.Handler) http.Handler {
		handlers := make(map[string]http.Handler, len(byKind))
		for kind, mw := range byKind {
			handlers[kind] = mw(next)
		}
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			serveVerified(handlers, w, r)
		})
	}, nil
}

// buildKindMiddlewares validates the kinds and environment configuration
// and builds one SDK callback middleware per allowed token kind.
//
// Parameters: kinds are the expected token typ values; at least one is
// required and each must be a gateway-signed kind.
// Returns: the per-kind middleware builders and nil, or an error naming
// only configuration facts.
func buildKindMiddlewares(kinds []string) (map[string]func(http.Handler) http.Handler, error) {
	if len(kinds) == 0 {
		return nil, errors.New("gateway: verify middleware: at least one token kind is required")
	}
	verifier, err := newCallbackVerifier()
	if err != nil {
		return nil, err
	}
	byKind := make(map[string]func(http.Handler) http.Handler, len(kinds))
	for _, kind := range kinds {
		kind = strings.TrimSpace(kind)
		if err := requireGatewayKind(kind); err != nil {
			return nil, err
		}
		if _, dup := byKind[kind]; !dup {
			byKind[kind] = verify.Middleware(verifier, kind)
		}
	}
	return byKind, nil
}

// newCallbackVerifier builds the SDK verifier for inbound callbacks: the
// gateway JWKS endpoint is APP_GATEWAY_URL/.well-known/jwks, aud must equal
// this party's id (APP_PARTY_ID), and webhook_id dedupe runs through the
// Redis-backed ReplayCache (T06) over the REDIS_* environment. It fails
// closed on an empty APP_GATEWAY_URL or APP_PARTY_ID before any I/O.
//
// Parameters: none; reads the APP_GATEWAY_URL, APP_PARTY_ID and REDIS_*
// environment variables.
// Returns: the verifier and nil, or an error naming only the bad variable.
func newCallbackVerifier() (*verify.Verifier, error) {
	rawURL := strings.TrimSpace(utils.GetEnvString("APP_GATEWAY_URL", ""))
	if rawURL == "" {
		return nil, errors.New("gateway: verify middleware: APP_GATEWAY_URL is empty")
	}
	self := strings.TrimSpace(utils.GetEnvString("APP_PARTY_ID", ""))
	if self == "" {
		return nil, errors.New("gateway: verify middleware: APP_PARTY_ID is empty")
	}
	verifier, err := verify.NewVerifier(
		strings.TrimRight(rawURL, "/")+jwksPath,
		self,
		verify.WithReplayCache(NewReplayCache(newReplayRedisClient())),
	)
	if err != nil {
		return nil, fmt.Errorf("gateway: verify middleware: %w", err)
	}
	return verifier, nil
}

// requireGatewayKind rejects a kind that the wire Token Kinds table does
// not list as gateway-signed (unknown typ and ks-party+jwt alike), so the
// middleware can only ever verify tokens the gateway itself signs.
//
// Parameters: kind is the trimmed token typ value.
// Returns: nil when the kind is gateway-signed, else an error naming the
// rejected kind (never any key material).
func requireGatewayKind(kind string) error {
	row, ok := wire.KindForTyp(kind)
	if !ok || row.Signer != wire.SignerGateway {
		return fmt.Errorf(
			"gateway: verify middleware: %q is not a gateway-signed token kind", kind)
	}
	return nil
}

// serveVerified dispatches one request to the handler of its token kind.
// A missing or malformed bearer token, or a typ outside the allowed kinds,
// denies 401 with the SDK's denial body before any handler runs; an allowed
// typ is delegated to that kind's SDK middleware, which performs the
// signature, claims, webhook-id match and replay checks.
//
// Parameters: handlers maps each allowed typ to its wrapped handler; w is
// the response writer; r is the inbound callback request.
// Returns: nothing; every failure path writes 401 without calling next.
func serveVerified(handlers map[string]http.Handler, w http.ResponseWriter, r *http.Request) {
	token, ok := bearerToken(r)
	if !ok {
		denyUnauthorized(w)
		return
	}
	hdr, err := decodeHeader(token)
	if err != nil {
		denyUnauthorized(w)
		return
	}
	handler, ok := handlers[hdr.Typ]
	if !ok {
		denyUnauthorized(w)
		return
	}
	handler.ServeHTTP(w, r)
}

// ClaimsFromContext returns the verified claims VerifyMiddleware stored on
// the request context (purpose, sub, service, listing_id, webhook id, ...),
// or the zero verify.Claims when the context never passed through the
// middleware. The returned value is a copy from the SDK; mutating it never
// affects the middleware.
//
// Parameters: ctx is the request context handed to the wrapped handler.
// Returns: the verified claims of the token that authorized the request.
func ClaimsFromContext(ctx context.Context) verify.Claims {
	return verify.ClaimsFromContext(ctx)
}

// bearerToken extracts the credentials of an Authorization header. The
// scheme compare is case-insensitive (RFC 7235); a missing header, a
// scheme other than Bearer, or empty credentials yield ok=false.
//
// Parameters: r is the inbound callback request.
// Returns: the bearer token and true, or false when no bearer credentials
// are present.
func bearerToken(r *http.Request) (string, bool) {
	const prefix = "Bearer "
	h := r.Header.Get("Authorization")
	if len(h) <= len(prefix) || !strings.EqualFold(h[:len(prefix)], prefix) {
		return "", false
	}
	tok := strings.TrimSpace(h[len(prefix):])
	return tok, tok != ""
}

// denyUnauthorized writes the same 401 denial body the SDK middleware
// writes, so a dispatch failure and a verification failure are
// indistinguishable to the gateway.
//
// Parameters: w is the response writer to deny on.
// Returns: nothing.
func denyUnauthorized(w http.ResponseWriter) {
	http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
}

// newReplayRedisClient builds the Redis client the webhook_id replay
// reservations (the T06 ReplayCache) live in, from REDIS_HOST, REDIS_PORT
// and REDIS_PASSWORD with the same defaults as the driver config
// (localhost:6379, no password). The client is not pinged here — the
// ReplayCache fails closed per request when Redis is unreachable.
//
// Parameters: none; reads the REDIS_* environment variables.
// Returns: a ready-to-use Redis client. Never panics.
func newReplayRedisClient() *redis.Client {
	return redis.NewClient(&redis.Options{
		Addr: utils.GetEnvString("REDIS_HOST", "localhost") + ":" +
			utils.GetEnvString("REDIS_PORT", "6379"),
		Password: utils.GetEnvString("REDIS_PASSWORD", ""),
	})
}
