#!/usr/bin/env bash
#
# Removes the Konsulin API suite's seeded FHIR resources from a dev Blaze,
# directly against Blaze (no gateway, no auth). Safe against a Blaze that also
# holds real QA data: everything deleted is keyed on the suite's fixed seed ids
# (Phase A) or on content only the suite's seeds carry (Phase B legacy litter).
# Real data is never matched:
#   - Organization/Konsulin (boot-created) is not a seed id and its name is not
#     "Konsulin Demo Clinic", so it is never deleted;
#   - Questionnaire/soap is not a seed id and Phase B matches only the wellbeing
#     questionnaire title, so the real SOAP questionnaire is never touched;
#   - canonical references (e.g. QuestionnaireResponse.questionnaire) are not
#     enforced by Blaze, so patient data never blocks a seed delete.
#
# A failed bru run leaves ResearchSubject/Slot/Appointment resources that
# reference the seed ids (fixed ids are shared across runs); deleting a
# referenced seed would 409, so Phase A also deletes resources that directly
# reference a seed. Anything else (e.g. a QA location referencing a legacy
# demo-clinic org) blocks the delete and the resource is skipped with a warning.
#
# Usage:
#   scripts/bru-cleanup.sh
#   BLAZE_BASE_URL=http://localhost:8080 scripts/bru-cleanup.sh
#
# Idempotent: re-running is a no-op. Exits 0 unless Blaze is unreachable.

BLAZE_BASE_URL="${BLAZE_BASE_URL:-http://localhost:8080}"
FHIR="${BLAZE_BASE_URL%/}/fhir"

DELETED=0
SKIPPED=0

# delete_ref <Type>/<id> — best-effort DELETE; 2xx counts as deleted, 404/410
# as already absent, 409 as skipped (still referenced), anything else warned.
delete_ref() {
  ref="$1"
  code="$(curl -s -o /dev/null -w '%{http_code}' -X DELETE "${FHIR}/${ref}")"
  case "$code" in
    200|201|204) DELETED=$((DELETED + 1)) ;;
    404|410) : ;;
    409) SKIPPED=$((SKIPPED + 1)); echo "  skip ${ref}: still referenced" ;;
    *) echo "  WARN: DELETE ${ref} -> ${code}" ;;
  esac
}

# ids <Type> <param> <value> — ids of resources matching param=value (single
# page capped at 1000; test litter never exceeds that).
ids() {
  curl -s -G "${FHIR}/$1" --data-urlencode "$2=$3" --data-urlencode "_count=1000" \
    | jq -r '.entry[]?.resource.id // empty'
}

# delete_each <Type> <param> <value> — delete every matching resource.
delete_each() {
  for id in $(ids "$@"); do
    delete_ref "$1/${id}"
  done
}

# delete_slot_graph <id> — appointments referencing the slot first, then it.
delete_slot_graph() {
  delete_each Appointment slot "Slot/$1"
  delete_ref "Slot/$1"
}

# delete_schedule_graph <id> — slots (and their appointments) first, then the
# schedule; also any appointment referencing the schedule directly.
delete_schedule_graph() {
  delete_each Appointment actor "Schedule/$1"
  for id in $(ids Slot schedule "Schedule/$1"); do
    delete_slot_graph "$id"
  done
  delete_ref "Schedule/$1"
}

# delete_role_graph <id> — schedules and appointments referencing the role
# first, then the role itself.
delete_role_graph() {
  delete_each Appointment actor "PractitionerRole/$1"
  for id in $(ids Schedule actor "PractitionerRole/$1"); do
    delete_schedule_graph "$id"
  done
  delete_ref "PractitionerRole/$1"
}

# delete_org_graph <id> — roles referencing the org first, then the org.
# Locations and HealthcareServices are NOT looked up here: Blaze ignores the
# managingOrganization / providedBy search params (it returns every resource of
# that type regardless of the value), so sweeping by them could delete real QA
# data. Phase B finds legacy locations/healthcare services by their distinctive
# names instead; Phase A deletes the fixed-id seed location/healthcare service
# explicitly. Non-seed references (real QA data) are left alone; if they still
# block the org delete, the org is skipped and reported.
delete_org_graph() {
  for id in $(ids PractitionerRole organization "Organization/$1"); do
    delete_role_graph "$id"
  done
  delete_ref "Organization/$1"
}

# delete_study_graph <id> — research subjects referencing the study first.
delete_study_graph() {
  delete_each ResearchSubject study "ResearchStudy/$1"
  delete_ref "ResearchStudy/$1"
}

# phase_a_fixed_ids — the suite's current seed ids (post-idempotent conversion)
# plus every resource that directly references them.
phase_a_fixed_ids() {
  echo "[phase A] fixed seed ids"
  delete_study_graph seed-study
  delete_schedule_graph seed-schedule
  delete_role_graph seed-role
  delete_each Appointment actor "HealthcareService/seed-hs"
  delete_each Appointment actor "Location/seed-location"
  delete_each Appointment actor "Organization/seed-clinic"
  delete_ref "PlanDefinition/seed-protocol"
  delete_ref "Questionnaire/seed-wellbeing"
  delete_ref "Questionnaire/seed-soap"
  delete_ref "HealthcareService/seed-hs"
  delete_ref "Location/seed-location"
  delete_org_graph seed-clinic
}

# phase_b_legacy — pre-conversion random-id litter, keyed on the seeds'
# distinctive content so real QA data (different names/titles) is never matched.
phase_b_legacy() {
  echo "[phase B] legacy content-keyed litter"
  for id in $(ids ResearchStudy title "Wellbeing Monitoring Study"); do
    delete_study_graph "$id"
  done
  for id in $(ids PlanDefinition title "Wellbeing Check-in Protocol"); do
    delete_ref "PlanDefinition/${id}"
  done
  for id in $(ids Questionnaire title "Konsulin General Wellbeing Assessment"); do
    delete_ref "Questionnaire/${id}"
  done
  # Healthcare services before locations: legacy services reference the seed
  # locations, so deleting a location while its service still exists would 409.
  for id in $(ids HealthcareService name "Konsulin Tele-Consultation"); do
    delete_ref "HealthcareService/${id}"
  done
  for id in $(ids Location name "Konsulin Demo Clinic — Main"); do
    delete_ref "Location/${id}"
  done
  for id in $(ids Organization name "Konsulin Demo Clinic"); do
    delete_ref "Organization/${id}"
  done
}

main() {
  echo "Blaze: ${FHIR}"
  if ! curl -sf --max-time 5 "${FHIR}/metadata" > /dev/null; then
    echo "ERROR: Blaze unreachable at ${FHIR}" >&2
    exit 1
  fi
  phase_a_fixed_ids
  phase_b_legacy
  echo "cleanup finished: ${DELETED} deleted, ${SKIPPED} skipped (still referenced)"
}

main "$@"
