#!/bin/sh
# gateway-bootstrap.sh — bootstrap the dev-mode federation gateway for CI.
#
# Against a gateway running in development mode (TOTP second factor unset,
# so the operator login is password-only: no TOTP field is ever sent), this
# script:
#   1. logs in as the operator (POST /v1/admin/auth/login);
#   2. creates the backend party and the automation (stub) party
#      (POST /v1/admin/parties);
#   3. issues a single-use enrollment token for each party
#      (POST /v1/admin/parties/{id}/enrollment-token);
#   4. enrolls both parties with proof-of-possession keys
#      (POST /v1/parties/{id}/enroll);
#   5. grants the automation party the listing:enlist capability and
#      enlists the stub listings (POST /v1/listings): send-magiclink,
#      notify-provider, modify-profile, update-avatar, interpret,
#      send-wa-link — all pointing at the CI stub server;
#   6. prints the backend APP_* values the CI app service needs, and
#      appends them to $GITHUB_ENV when running under GitHub Actions.
#
# Inputs (environment):
#   GATEWAY_URL         gateway base URL (default http://localhost:8080)
#   STUB_BASE_URL       CI stub base URL (default http://localhost:8081)
#   OPERATOR_USERNAME   operator login name (default operator)
#   OPERATOR_PASSWORD   operator password (required; must not contain
#                       double quotes or backslashes — it is embedded in
#                       a JSON body)
#
# Output (stdout and $GITHUB_ENV): APP_GATEWAY_URL, APP_PARTY_ID,
# APP_PARTY_PRIVATE_KEY, APP_PARTY_ENC_PRIVATE_KEY,
# APP_PARTY_ENROLLMENT_TOKEN, APP_AUTOMATION_PARTY_ID.
#
# Requires: POSIX sh, curl, openssl 3+ (Ed25519 pkeyutl -rawin).
set -eu

GATEWAY_URL=${GATEWAY_URL:-http://localhost:8080}
STUB_BASE_URL=${STUB_BASE_URL:-http://localhost:8081}
OPERATOR_USERNAME=${OPERATOR_USERNAME:-operator}
OPERATOR_PASSWORD=${OPERATOR_PASSWORD:-}

die() { printf 'gateway-bootstrap: %s\n' "$*" >&2; exit 1; }
log() { printf 'gateway-bootstrap: %s\n' "$*" >&2; }

[ -n "$OPERATOR_PASSWORD" ] || die "OPERATOR_PASSWORD is required"
case $OPERATOR_PASSWORD in
  *\"*|*\\*) die "OPERATOR_PASSWORD must not contain quotes or backslashes" ;;
esac
command -v curl >/dev/null 2>&1 || die "curl is required"
command -v openssl >/dev/null 2>&1 || die "openssl is required"

TMP_DIR=$(mktemp -d)
trap 'rm -rf "$TMP_DIR"' 0
trap 'exit 1' 1 2 15

# b64 / b64url read stdin and print the encoded form on one line.
b64() { openssl base64 | tr -d '\n'; }
b64url() { openssl base64 | tr '+/' '-_' | tr -d '=\n'; }

# json_field KEY JSON — prints the first string value of KEY.
json_field() {
  printf '%s\n' "$2" |
    sed -n "s/.*\"$1\"[[:space:]]*:[[:space:]]*\"\\([^\"]*\\)\".*/\\1/p" |
    sed -n '1p'
}

# request METHOD PATH BODY AUTH — one gateway POST; sets REQ_STATUS and
# REQ_BODY and dies on any non-2xx answer (surfacing the error envelope).
request() {
  _method=$1 _path=$2 _body=$3 _auth=$4
  _h=$(openssl rand -hex 16)
  _idem=$(printf '%s-%s-%s-%s-%s' \
    "$(printf '%s' "$_h" | cut -c1-8)" "$(printf '%s' "$_h" | cut -c9-12)" \
    "$(printf '%s' "$_h" | cut -c13-16)" "$(printf '%s' "$_h" | cut -c17-20)" \
    "$(printf '%s' "$_h" | cut -c21-32)")
  if [ -n "$_auth" ]; then
    set -- -H "Authorization: Bearer $_auth"
  else
    set --
  fi
  REQ_STATUS=$(curl -sS -o "$TMP_DIR/resp.json" -w '%{http_code}' \
    -X "$_method" -H 'Content-Type: application/json' \
    -H "Idempotency-Key: $_idem" --data-binary "$_body" \
    "$@" "$GATEWAY_URL$_path") || die "curl failed: $_path"
  REQ_BODY=$(cat "$TMP_DIR/resp.json")
  case $REQ_STATUS in
    2[0-9][0-9]) ;;
    *) die "$_path failed (HTTP $REQ_STATUS): $REQ_BODY" ;;
  esac
}

# sign_jwt HEADER CLAIMS KEYFILE — Ed25519-signs the compact JWS with
# openssl (raw Ed25519, OpenSSL 3+); prints the token.
sign_jwt() {
  _input=$(printf '%s.%s' \
    "$(printf '%s' "$1" | b64url)" "$(printf '%s' "$2" | b64url)")
  printf '%s' "$_input" > "$TMP_DIR/jwt.msg"
  openssl pkeyutl -sign -inkey "$3" -rawin \
    -in "$TMP_DIR/jwt.msg" -out "$TMP_DIR/jwt.sig" >/dev/null 2>&1 ||
    die "openssl pkeyutl -sign failed (OpenSSL 3+ with Ed25519 required)"
  printf '%s.%s' "$_input" "$(b64url < "$TMP_DIR/jwt.sig")"
}

# gen_party_keys ROLE — generates an Ed25519 + X25519 pair for ROLE and
# stores ROLE_SEED, ROLE_PUB, ROLE_PUBURL, ROLE_KID, ROLE_ENC_SEED,
# ROLE_ENC_PUB (SDK registration encodings: std base64) and ROLE_KEYFILE.
gen_party_keys() {
  _role=$1
  _up=$(printf '%s' "$_role" | tr '[:lower:]' '[:upper:]')
  _sig="$TMP_DIR/$_role-sig.pem"
  _enc="$TMP_DIR/$_role-enc.pem"
  openssl genpkey -algorithm ed25519 -out "$_sig" >/dev/null 2>&1 ||
    die "openssl genpkey ed25519 failed"
  openssl genpkey -algorithm x25519 -out "$_enc" >/dev/null 2>&1 ||
    die "openssl genpkey x25519 failed"
  # PKCS#8/SPKI DERs end with the raw 32-byte key material.
  _seed=$(openssl pkey -in "$_sig" -outform DER | tail -c 32 | b64)
  _pub=$(openssl pkey -in "$_sig" -pubout -outform DER | tail -c 32 | b64)
  _puburl=$(openssl pkey -in "$_sig" -pubout -outform DER | tail -c 32 | b64url)
  _encseed=$(openssl pkey -in "$_enc" -outform DER | tail -c 32 | b64)
  _encpub=$(openssl pkey -in "$_enc" -pubout -outform DER | tail -c 32 | b64)
  # RFC 7638 thumbprint: SHA-256 over the canonical OKP JWK JSON.
  _kid=$(printf '%s' "{\"crv\":\"Ed25519\",\"kty\":\"OKP\",\"x\":\"$_puburl\"}" |
    openssl dgst -sha256 -binary | b64url)
  eval "$_up"'_SEED=$_seed'
  eval "$_up"'_PUB=$_pub'
  eval "$_up"'_PUBURL=$_puburl'
  eval "$_up"'_KID=$_kid'
  eval "$_up"'_ENC_SEED=$_encseed'
  eval "$_up"'_ENC_PUB=$_encpub'
  eval "$_up"'_KEYFILE=$_sig'
}

# create_party ROLE ORG_NAME — creates a tier-authorized party and prints
# its party_id (created in state awaiting_key).
create_party() {
  _rand=$(openssl rand -hex 6)
  request POST /v1/admin/parties \
    "{\"org_name\":\"$2\",\"org_identifier\":\"oid-ci-$1-$_rand\",\"tier\":\"authorized\"}" \
    "$ADMIN_TOKEN"
  _pid=$(json_field party_id "$REQ_BODY")
  [ -n "$_pid" ] || die "create party $1 returned no party_id"
  printf '%s' "$_pid"
}

# issue_enrollment_token PARTY_ID — prints the single-use plaintext token.
issue_enrollment_token() {
  request POST "/v1/admin/parties/$1/enrollment-token" '{}' "$ADMIN_TOKEN"
  _tok=$(json_field enrollment_token "$REQ_BODY")
  [ -n "$_tok" ] || die "no enrollment token issued for $1"
  printf '%s' "$_tok"
}

# enroll_party ROLE PARTY_ID TOKEN — redeems the enrollment token with a
# proof-of-possession ks-party+jwt bound to the exact body bytes.
enroll_party() {
  _pid=$2 _tok=$3
  eval '_pub=$'"$(printf '%s' "$1" | tr '[:lower:]' '[:upper:]')"'_PUB'
  eval '_kid=$'"$(printf '%s' "$1" | tr '[:lower:]' '[:upper:]')"'_KID'
  eval '_encpub=$'"$(printf '%s' "$1" | tr '[:lower:]' '[:upper:]')"'_ENC_PUB'
  eval '_keyfile=$'"$(printf '%s' "$1" | tr '[:lower:]' '[:upper:]')"'_KEYFILE'
  _now=$(date +%s)
  _exp=$((_now + 300))
  _body=$(printf '{"enrollment_token":"%s","public_key":"%s","kid":"%s","alg":"Ed25519","enc_public_key":"%s"}' \
    "$_tok" "$_pub" "$_kid" "$_encpub")
  _bhash=$(printf '%s' "$_body" | openssl dgst -sha256 -binary | b64url)
  _jti=$(openssl rand -base64 16 | tr '+/' '-_' | tr -d '=\n')
  _hdr=$(printf '{"alg":"EdDSA","kid":"%s","typ":"ks-party+jwt"}' "$_kid")
  _claims=$(printf '{"aud":"gateway","exp":%s,"iat":%s,"iss":"%s","jti":"%s","body_sha256":"%s"}' \
    "$_exp" "$_now" "$_pid" "$_jti" "$_bhash")
  _proof=$(sign_jwt "$_hdr" "$_claims" "$_keyfile")
  request POST "/v1/parties/$_pid/enroll" "$_body" "$_proof"
  log "enrolled party $_pid"
}

# mint_party_jwt ROLE PARTY_ID — mints a short-lived ks-party+jwt the
# party uses to authenticate its own API calls (fresh jti per mint).
mint_party_jwt() {
  _up=$(printf '%s' "$1" | tr '[:lower:]' '[:upper:]')
  eval '_kid=$'"$_up"'_KID'
  eval '_keyfile=$'"$_up"'_KEYFILE'
  _now=$(date +%s)
  _exp=$((_now + 300))
  _jti=$(openssl rand -base64 16 | tr '+/' '-_' | tr -d '=\n')
  _hdr=$(printf '{"alg":"EdDSA","kid":"%s","typ":"ks-party+jwt"}' "$_kid")
  _claims=$(printf '{"aud":"gateway","exp":%s,"iat":%s,"iss":"%s","jti":"%s"}' \
    "$_exp" "$_now" "$2" "$_jti")
  sign_jwt "$_hdr" "$_claims" "$_keyfile"
}

# enlist NAME ENDPOINT — creates one free webhook listing. Mints a fresh
# automation party JWT per call (the gateway treats each jti as single-use).
enlist() {
  _jwt=$(mint_party_jwt automation "$AUTOMATION_PID")
  _body=$(printf '{"name":"%s","description":"CI stub service %s","endpoint_url":"%s","price":0,"invocation":"webhook","price_scope":"party"}' \
    "$1" "$1" "$2")
  request POST /v1/listings "$_body" "$_jwt"
  _lid=$(json_field listing_id "$REQ_BODY")
  [ -n "$_lid" ] || die "enlist $1 returned no listing_id"
  log "enlisted listing $1 -> $2 ($_lid)"
}

# emit NAME VALUE — prints one APP_* export line and mirrors it into
# $GITHUB_ENV when running under GitHub Actions.
emit() {
  printf '%s=%s\n' "$1" "$2"
  if [ -n "${GITHUB_ENV:-}" ]; then
    printf '%s=%s\n' "$1" "$2" >> "$GITHUB_ENV"
  fi
}

log "logging in as operator $OPERATOR_USERNAME (development mode: no TOTP)"
request POST /v1/admin/auth/login \
  "{\"username\":\"$OPERATOR_USERNAME\",\"password\":\"$OPERATOR_PASSWORD\"}" ""
ADMIN_TOKEN=$(json_field token "$REQ_BODY")
[ -n "$ADMIN_TOKEN" ] || die "login returned no admin token"
log "operator login ok"

gen_party_keys backend
gen_party_keys automation

BACKEND_PID=$(create_party backend "Konsulin Backend CI")
log "created backend party $BACKEND_PID"
AUTOMATION_PID=$(create_party automation "Konsulin Automation Stub CI")
log "created automation (stub) party $AUTOMATION_PID"

BACKEND_TOKEN=$(issue_enrollment_token "$BACKEND_PID")
log "issued enrollment token for backend party"
AUTOMATION_TOKEN=$(issue_enrollment_token "$AUTOMATION_PID")
log "issued enrollment token for automation party"

enroll_party backend "$BACKEND_PID" "$BACKEND_TOKEN"
enroll_party automation "$AUTOMATION_PID" "$AUTOMATION_TOKEN"

# listing:enlist is not an authorized-tier default: grant it explicitly so
# the automation party can enlist its stub listings.
request POST "/v1/admin/parties/$AUTOMATION_PID/capabilities" \
  '{"capability":"listing:enlist"}' "$ADMIN_TOKEN"
log "granted listing:enlist to $AUTOMATION_PID"

enlist send-magiclink "$STUB_BASE_URL/magiclink/send-magiclink"
enlist notify-provider "$STUB_BASE_URL/provider/invoke"
enlist modify-profile "$STUB_BASE_URL/provider/invoke"
enlist update-avatar "$STUB_BASE_URL/provider/invoke"
enlist interpret "$STUB_BASE_URL/provider/invoke"
enlist send-wa-link "$STUB_BASE_URL/provider/invoke"

emit APP_GATEWAY_URL "$GATEWAY_URL"
emit APP_PARTY_ID "$BACKEND_PID"
emit APP_PARTY_PRIVATE_KEY "$BACKEND_SEED"
emit APP_PARTY_ENC_PRIVATE_KEY "$BACKEND_ENC_SEED"
emit APP_PARTY_ENROLLMENT_TOKEN "$BACKEND_TOKEN"
emit APP_AUTOMATION_PARTY_ID "$AUTOMATION_PID"
log "gateway bootstrap complete"
