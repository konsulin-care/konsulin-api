#!/usr/bin/env bash
#
# Regression test for scripts/bru-cleanup.sh against a live Blaze.
#
# Usage:
#   scripts/test-bru-cleanup.sh [BLAZE_BASE_URL]
#
# Seeds the fixed-ID seed tree plus a failed-run referencing chain
# (ResearchSubject -> study, Slot -> schedule, Appointment -> slot/role) and a
# legacy content-matched resource (random id, seed title), runs the cleanup
# twice (idempotency), then asserts:
#   - every seed resource and direct referencing resource is gone,
#   - the boot Organization/Konsulin is preserved,
#   - Questionnaire/soap is untouched (real data: present stays present,
#     absent stays absent).
#
# Exits non-zero when any assertion fails. Requires jq and curl.

BLAZE_BASE_URL="${1:-http://localhost:8080}"
FHIR="${BLAZE_BASE_URL%/}/fhir"

FAILED=0

# code <Type>/<id> — HTTP status of a GET.
code() {
  curl -s -o /dev/null -w '%{http_code}' "${FHIR}/$1"
}

# put <Type>/<id> <json> — PUT a fixture resource; marks FAILED on error.
put() {
  http="$(curl -s -o /dev/null -w '%{http_code}' -X PUT "${FHIR}/$1" \
    -H 'content-type: application/fhir+json' -d "$2")"
  if [ "$http" != "200" ] && [ "$http" != "201" ]; then
    echo "FAIL: setup PUT $1 -> $http"
    FAILED=1
  fi
}

# check_absent <label> <Type>/<id> — 404/410 means gone.
check_absent() {
  c="$(code "$2")"
  if [ "$c" = "404" ] || [ "$c" = "410" ]; then
    echo "PASS: $1 absent ($c)"
  else
    echo "FAIL: $1 present ($c)"
    FAILED=1
  fi
}

# check_code <label> <expected> <Type>/<id>
check_code() {
  c="$(code "$3")"
  if [ "$c" = "$2" ]; then
    echo "PASS: $1 ($c)"
  else
    echo "FAIL: $1 expected $2, got $c"
    FAILED=1
  fi
}

echo "== setup fixtures =="
put Organization/seed-clinic '{"resourceType":"Organization","id":"seed-clinic","active":true,"name":"Konsulin Demo Clinic"}'
put Location/seed-location '{"resourceType":"Location","id":"seed-location","status":"active","name":"Konsulin Demo Clinic — Main","managingOrganization":{"reference":"Organization/seed-clinic"}}'
put HealthcareService/seed-hs '{"resourceType":"HealthcareService","id":"seed-hs","active":true,"name":"Konsulin Tele-Consultation","providedBy":{"reference":"Organization/seed-clinic"},"location":[{"reference":"Location/seed-location"}]}'
put PractitionerRole/seed-role '{"resourceType":"PractitionerRole","id":"seed-role","active":true,"organization":{"reference":"Organization/seed-clinic"}}'
put Schedule/seed-schedule '{"resourceType":"Schedule","id":"seed-schedule","active":true,"actor":[{"reference":"PractitionerRole/seed-role"}]}'
put Questionnaire/seed-wellbeing '{"resourceType":"Questionnaire","id":"seed-wellbeing","status":"active","title":"Konsulin General Wellbeing Assessment"}'
put Questionnaire/seed-soap '{"resourceType":"Questionnaire","id":"seed-soap","status":"active","title":"SOAP Consultation Notes"}'
put PlanDefinition/seed-protocol '{"resourceType":"PlanDefinition","id":"seed-protocol","status":"active","title":"Wellbeing Check-in Protocol"}'
put ResearchStudy/seed-study '{"resourceType":"ResearchStudy","id":"seed-study","status":"active","title":"Wellbeing Monitoring Study","protocol":[{"reference":"PlanDefinition/seed-protocol"}]}'
put Patient/zz-cleanup-pat '{"resourceType":"Patient","id":"zz-cleanup-pat","active":true}'
put ResearchSubject/zz-cleanup-rs '{"resourceType":"ResearchSubject","id":"zz-cleanup-rs","status":"on-study","study":{"reference":"ResearchStudy/seed-study"},"individual":{"reference":"Patient/zz-cleanup-pat"}}'
put Slot/zz-cleanup-slot '{"resourceType":"Slot","id":"zz-cleanup-slot","status":"busy","schedule":{"reference":"Schedule/seed-schedule"},"start":"2026-01-02T03:00:00Z","end":"2026-01-02T03:30:00Z"}'
put Appointment/zz-cleanup-appt '{"resourceType":"Appointment","id":"zz-cleanup-appt","status":"booked","slot":[{"reference":"Slot/zz-cleanup-slot"}],"participant":[{"actor":{"reference":"PractitionerRole/seed-role"},"status":"accepted"},{"actor":{"reference":"Patient/zz-cleanup-pat"},"status":"accepted"}]}'
put ResearchStudy/zz-legacy-study '{"resourceType":"ResearchStudy","id":"zz-legacy-study","status":"active","title":"Wellbeing Monitoring Study"}'
put ResearchSubject/zz-legacy-rs '{"resourceType":"ResearchSubject","id":"zz-legacy-rs","status":"on-study","study":{"reference":"ResearchStudy/zz-legacy-study"},"individual":{"reference":"Patient/zz-cleanup-pat"}}'
# Real-data stand-ins: a non-seed organization and a location referencing it
# must survive the cleanup untouched (guards against type-wide sweeps).
put Organization/zz-qa-org '{"resourceType":"Organization","id":"zz-qa-org","active":true,"name":"Cabang Klinik QA"}'
put Location/zz-qa-location '{"resourceType":"Location","id":"zz-qa-location","status":"active","name":"Cabang Klinik QA — Main","managingOrganization":{"reference":"Organization/zz-qa-org"}}'
if [ "$FAILED" = "1" ]; then
  echo "ABORT: fixture setup failed; refusing to run the cleanup against a broken baseline"
  exit 1
fi

echo "== run scripts/bru-cleanup.sh (first pass) =="
sh scripts/bru-cleanup.sh

echo "== assertions after first pass =="
check_absent "seed-clinic org" Organization/seed-clinic
check_absent "seed-location" Location/seed-location
check_absent "seed-hs" HealthcareService/seed-hs
check_absent "seed-role" PractitionerRole/seed-role
check_absent "seed-schedule" Schedule/seed-schedule
check_absent "seed-wellbeing" Questionnaire/seed-wellbeing
check_absent "seed-soap" Questionnaire/seed-soap
check_absent "seed-protocol" PlanDefinition/seed-protocol
check_absent "seed-study" ResearchStudy/seed-study
check_absent "leftover research subject" ResearchSubject/zz-cleanup-rs
check_absent "leftover slot" Slot/zz-cleanup-slot
check_absent "leftover appointment" Appointment/zz-cleanup-appt
check_absent "legacy study" ResearchStudy/zz-legacy-study
check_absent "legacy research subject" ResearchSubject/zz-legacy-rs
check_code "boot Organization/Konsulin preserved" 200 Organization/Konsulin
check_code "QA org preserved" 200 Organization/zz-qa-org
check_code "QA location preserved" 200 Location/zz-qa-location

SOAP_BEFORE="$(code Questionnaire/soap)"
echo "== run scripts/bru-cleanup.sh (second pass, idempotency) =="
sh scripts/bru-cleanup.sh
SOAP_AFTER="$(code Questionnaire/soap)"
if [ "$SOAP_BEFORE" = "$SOAP_AFTER" ]; then
  echo "PASS: Questionnaire/soap untouched across both passes ($SOAP_BEFORE)"
else
  echo "FAIL: Questionnaire/soap changed $SOAP_BEFORE -> $SOAP_AFTER"
  FAILED=1
fi
check_absent "seed-study still absent after second pass" ResearchStudy/seed-study

# The cleanup does not own session-bound profiles; remove the harness patient
# and QA stand-ins.
curl -s -o /dev/null -X DELETE "${FHIR}/Patient/zz-cleanup-pat"
curl -s -o /dev/null -X DELETE "${FHIR}/Location/zz-qa-location"
curl -s -o /dev/null -X DELETE "${FHIR}/Organization/zz-qa-org"

if [ "$FAILED" = "1" ]; then
  echo "FAILED: one or more cleanup assertions did not hold"
  exit 1
fi
echo "ALL PASS"
