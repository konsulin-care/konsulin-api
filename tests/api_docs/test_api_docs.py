"""Validation suite for the Bruno API collection (docs/api).

Spec source: the finalized implementation plan for docs/api/fhir
(auth plumbing, role switching, superadmin seeding, patient/practitioner
journeys, admin, cleanup, reference chain).

Run with:
    python3 -m pytest tests/api_docs/test_api_docs.py -v
"""

import os
import re
import yaml

REPO_ROOT = os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
API_DIR = os.path.join(REPO_ROOT, "docs", "api")
FHIR_DIR = os.path.join(API_DIR, "fhir")

# ---------------------------------------------------------------- structure

EXPECTED_FHIR_TREE = {
    "seed": [
        "seed-organization.yml",
        "seed-location.yml",
        "seed-healthcare-service.yml",
        "seed-practitioner-role.yml",
        "seed-schedule.yml",
        "seed-questionnaire.yml",
        "seed-soap-questionnaire.yml",
        "seed-plan-definition.yml",
        "seed-research-study.yml",
    ],
    "patient": [
        "get-profile.yml",
        "list-active-research-studies.yml",
        "record-study-consent.yml",
        "get-research-progress.yml",
        "list-questionnaires.yml",
        "get-completion-total.yml",
        "get-questionnaire-detail.yml",
        "submit-questionnaire-response.yml",
        "list-record-summary.yml",
        "create-journal.yml",
        "update-journal.yml",
        "list-upcoming-appointments.yml",
        "check-practitioner-availability.yml",
        "book-appointment.yml",
        "update-appointment-status.yml",
        "write-referral.yml",
        "get-circle-stats.yml",
    ],
    "practitioner": [
        "get-profile.yml",
        "get-roles.yml",
        "get-schedule.yml",
        "list-practitioner-slots.yml",
        "get-busy-slots.yml",
        "list-sessions.yml",
        "get-today-sessions.yml",
        "update-availability.yml",
        "create-soap-notes.yml",
    ],
    "admin": [
        "create-questionnaire-draft.yml",
        "update-questionnaire.yml",
    ],
    "cleanup": [
        "purge-journey-resources.yml",
        "delete-organization.yml",
        "delete-practitioner-profile.yml",
        "delete-patient-profile.yml",
    ],
    "reference": [
        "metadata.yml",
        "list-all-clinic-locations.yml",
        "list-practitioners-in-a-clinic.yml",
        "view-healthcare-service-provided-by-a-practitioner-in-a-clinic.yml",
    ],
}

ALLOWED_TAGS = {
    "appointment",
    "research",
    "assessment",
    "profile",
    "clinic",
    "record",
    "referral",
    "utility",
    "auth",
}

# Convention tests apply to the fhir collection and the auth folder only
# (the scope of this work). Other folders are pre-existing and untouched.
SCOPE_PREFIXES = (os.path.join("fhir", ""), os.path.join("auth", ""))

REQUIRED_OPENCOLLECTION_VARS = [
    "preAuthSessionId",
    "linkCode",
    "messageId",
    "sessionToken",
    "activeRole",
    "userId",
    "patientId",
    "practitionerId",
    "organizationId",
    "locationId",
    "healthcareServiceId",
    "practitionerRoleId",
    "scheduleId",
    "slotId",
    "appointmentId",
    "questionnaireId",
    "questionnaireResponseId",
    "journalId",
    "planDefinitionId",
    "researchStudyId",
]

# ---------------------------------------------------------------- chain spec

# name -> expected next request name (None = terminal / no chaining)
EXPECTED_CHAIN = {
    "Send Magic Link": "Check Email Exists",
    "Check Email Exists": "Create Anonymous Session",
    "Create Anonymous Session": "Consume Code",
    "Consume Code": "Claim Anonymous Resources",
    "Claim Anonymous Resources": "Set Active Role",
    "Set Active Role": "Seed Organization",
    "Seed Organization": "Seed Location",
    "Seed Location": "Seed Healthcare Service",
    "Seed Healthcare Service": "Seed Practitioner Role",
    "Seed Practitioner Role": "Seed Schedule",
    "Seed Schedule": "Seed Questionnaire",
    "Seed Questionnaire": "Seed SOAP Questionnaire",
    "Seed SOAP Questionnaire": "Seed Plan Definition",
    "Seed Plan Definition": "Seed Research Study",
    "Seed Research Study": "Patient: Get Profile",
    "Patient: Get Profile": "Patient: List Research Studies",
    "Patient: List Research Studies": "Patient: Record Study Consent",
    "Patient: Record Study Consent": "Patient: Get Research Progress",
    "Patient: Get Research Progress": "Patient: List Questionnaires",
    "Patient: List Questionnaires": "Patient: Get Completion Total",
    "Patient: Get Completion Total": "Patient: Get Questionnaire Detail",
    "Patient: Get Questionnaire Detail": "Patient: Submit Questionnaire Response",
    "Patient: Submit Questionnaire Response": "Patient: List Record Summary",
    "Patient: List Record Summary": "Patient: Create Journal",
    "Patient: Create Journal": "Patient: Update Journal",
    "Patient: Update Journal": "Patient: List Upcoming Appointments",
    "Patient: List Upcoming Appointments": "Patient: Check Practitioner Availability",
    "Patient: Check Practitioner Availability": "Patient: Book Appointment",
    "Patient: Book Appointment": "Patient: Update Appointment Status",
    "Patient: Update Appointment Status": "Patient: Write Referral",
    "Patient: Write Referral": "Patient: Get Circle Stats",
    "Patient: Get Circle Stats": "Practitioner: Get Profile",
    "Practitioner: Get Profile": "Practitioner: Get Roles",
    "Practitioner: Get Roles": "Practitioner: Get Schedule",
    "Practitioner: Get Schedule": "Practitioner: List Practitioner Slots",
    "Practitioner: List Practitioner Slots": "Practitioner: Get Busy Slots",
    "Practitioner: Get Busy Slots": "Practitioner: List Sessions",
    "Practitioner: List Sessions": "Practitioner: Get Today Sessions",
    "Practitioner: Get Today Sessions": "Practitioner: Update Availability",
    "Practitioner: Update Availability": "Practitioner: Create SOAP Notes",
    "Practitioner: Create SOAP Notes": "Admin: Create Questionnaire Draft",
    "Admin: Create Questionnaire Draft": "Admin: Update Questionnaire",
    "Admin: Update Questionnaire": "Cleanup: Purge Journey Resources",
    "Cleanup: Purge Journey Resources": "Cleanup: Delete Organization",
    "Cleanup: Delete Organization": "Cleanup: Delete Practitioner Profile",
    "Cleanup: Delete Practitioner Profile": "Cleanup: Delete Patient Profile",
    "Cleanup: Delete Patient Profile": "Sign Out",
    "Sign Out": None,
    "Switch Active Role": None,
    "FHIR Server Metadata": None,
    "List All Clinic Locations": "List Practitioners in a Clinic",
    "List Practitioners in a Clinic": "View Healthcare Service Provided by a Practitioner in a Clinic",
    "View Healthcare Service Provided by a Practitioner in a Clinic": None,
}

# ---------------------------------------------------------------- helpers


def all_yml_files():
    out = []
    for root, _dirs, files in os.walk(API_DIR):
        for f in files:
            if f.endswith(".yml") and f != "folder.yml":
                out.append(os.path.join(root, f))
    return sorted(out)


def load(path):
    with open(path, "r", encoding="utf-8") as fh:
        return yaml.safe_load(fh)


def request_docs():
    """Return {request_name: (relative_path, yaml_doc)} for all request files."""
    result = {}
    for path in all_yml_files():
        doc = load(path)
        rel = os.path.relpath(path, API_DIR)
        if not doc or doc.get("info", {}).get("type") != "http":
            continue
        name = doc["info"].get("name")
        if not name:
            raise AssertionError(f"{rel}: info.name is required")
        result[name] = (rel, doc)
    return result


def in_scope_docs():
    """Request docs under docs/api/fhir/** and docs/api/auth/**."""
    return {name: (rel, doc) for name, (rel, doc) in request_docs().items()
            if rel.startswith(SCOPE_PREFIXES)}


def scripts_of(doc, script_type):
    """Return the code strings of runtime scripts of a given type."""
    codes = []
    for entry in (doc.get("runtime") or {}).get("scripts") or []:
        if entry.get("type") == script_type and entry.get("code"):
            codes.append(entry["code"])
    return codes


def chained_targets(code):
    """Return non-null setNextRequest targets found in a script code block."""
    return re.findall(r'setNextRequest\("([^"]+)"\)', code)


def headers_of(doc):
    """Return header name->value map from http.headers and request.headers."""
    out = {}
    for block in ("http", "request"):
        for h in (doc.get(block) or {}).get("headers") or []:
            out[h.get("name")] = h.get("value")
    return out


def body_of(doc):
    return (doc.get("http") or {}).get("body") or {}


# ---------------------------------------------------------------- structure


def test_fhir_subfolder_tree():
    for folder, files in EXPECTED_FHIR_TREE.items():
        folder_dir = os.path.join(FHIR_DIR, folder)
        assert os.path.isdir(folder_dir), f"missing subfolder {folder}/"
        assert os.path.isfile(os.path.join(folder_dir, "folder.yml")), f"missing {folder}/folder.yml"
        present = sorted(f for f in os.listdir(folder_dir) if f.endswith(".yml") and f != "folder.yml")
        assert present == sorted(files), f"{folder}/ file mismatch: {present} != {sorted(files)}"


def test_root_folder_yml_exists():
    assert os.path.isfile(os.path.join(FHIR_DIR, "folder.yml"))


def test_subfolder_folder_ymls_valid():
    for folder in EXPECTED_FHIR_TREE:
        doc = load(os.path.join(FHIR_DIR, folder, "folder.yml"))
        assert doc["info"]["type"] == "folder"
        assert doc["info"]["name"] == folder
        assert isinstance(doc["info"].get("seq"), int)


def test_auth_switch_active_role_exists():
    assert os.path.isfile(os.path.join(API_DIR, "auth", "switch-active-role.yml"))


# ---------------------------------------------------------------- conventions


def test_every_request_has_conventions():
    for name, (rel, doc) in in_scope_docs().items():
        info = doc["info"]
        assert info.get("type") == "http", f"{rel}: info.type must be http"
        assert isinstance(info.get("seq"), int), f"{rel}: info.seq must be an int"
        tags = info.get("tags")
        assert isinstance(tags, list) and tags, f"{rel}: info.tags must be a non-empty list"
        assert "docs" in doc and str(doc["docs"]).strip(), f"{rel}: docs: block is required"
        assert isinstance(doc.get("settings"), dict), f"{rel}: settings: block is required"
        assert doc.get("http", {}).get("method"), f"{rel}: http.method is required"
        assert doc.get("http", {}).get("url"), f"{rel}: http.url is required"


def test_tags_are_allowed():
    for name, (rel, doc) in in_scope_docs().items():
        for tag in doc["info"]["tags"]:
            assert tag in ALLOWED_TAGS, f"{rel}: tag {tag!r} not in {sorted(ALLOWED_TAGS)}"


def test_seed_and_cleanup_carry_utility_tag():
    docs = in_scope_docs()
    for name in [f"Seed {n}" for n in ["Organization", "Location", "Healthcare Service",
                                       "Practitioner Role", "Schedule", "Questionnaire",
                                       "SOAP Questionnaire", "Plan Definition", "Research Study"]]:
        tags = docs[name][1]["info"]["tags"]
        assert "utility" in tags, f"{name}: seed requests must carry the utility tag"
    for name in ["Cleanup: Purge Journey Resources", "Cleanup: Delete Organization",
                 "Cleanup: Delete Practitioner Profile", "Cleanup: Delete Patient Profile"]:
        tags = docs[name][1]["info"]["tags"]
        assert "utility" in tags, f"{name}: cleanup requests must carry the utility tag"


def test_journey_requests_have_domain_tags_only():
    docs = in_scope_docs()
    # Cleanup profile deletions switch roles but are utility-tagged by design;
    # the no-utility rule applies to the patient/practitioner journeys only.
    for name in ROLE_SWITCH_REQUIRED:
        if name.startswith("Cleanup:"):
            continue
        tags = docs[name][1]["info"]["tags"]
        assert "utility" not in tags, f"{name}: journey requests must not carry the utility tag"
        assert tags, f"{name}: journey request must carry a business tag"


def test_admin_requests_tagged_assessment():
    docs = in_scope_docs()
    for name in ["Admin: Create Questionnaire Draft", "Admin: Update Questionnaire"]:
        tags = docs[name][1]["info"]["tags"]
        assert "assessment" in tags, f"{name}: admin questionnaire requests must be tagged assessment"


def test_request_names_unique():
    docs = request_docs()
    assert len(docs) == len(set(docs)), "duplicate info.name across the collection"


def test_fhir_chain_is_single_linear_sequence():
    """The fhir journey (seed->patient->practitioner->admin->cleanup) must be one
    connected chain: every request has exactly one non-null chained successor
    except the last one, and no forks."""
    docs = request_docs()
    chain_names = [
        "Set Active Role",
        "Seed Organization", "Seed Location", "Seed Healthcare Service",
        "Seed Practitioner Role", "Seed Schedule", "Seed Questionnaire",
        "Seed SOAP Questionnaire", "Seed Plan Definition", "Seed Research Study",
    ] + [f"Patient: {n}" for n in ["Get Profile", "List Research Studies",
        "Record Study Consent", "Get Research Progress", "List Questionnaires",
        "Get Completion Total", "Get Questionnaire Detail",
        "Submit Questionnaire Response", "List Record Summary", "Create Journal",
        "Update Journal", "List Upcoming Appointments",
        "Check Practitioner Availability", "Book Appointment",
        "Update Appointment Status", "Write Referral", "Get Circle Stats"]] \
        + [f"Practitioner: {n}" for n in ["Get Profile", "Get Roles", "Get Schedule",
        "List Practitioner Slots", "Get Busy Slots", "List Sessions",
        "Get Today Sessions", "Update Availability", "Create SOAP Notes"]] \
        + ["Admin: Create Questionnaire Draft", "Admin: Update Questionnaire",
           "Cleanup: Purge Journey Resources", "Cleanup: Delete Organization",
           "Cleanup: Delete Practitioner Profile", "Cleanup: Delete Patient Profile", "Sign Out"]
    for name in chain_names:
        rel, doc = docs[name]
        targets = [t for code in scripts_of(doc, "after-response") for t in chained_targets(code)]
        assert len(targets) <= 1, f"{rel}: expected at most one chained successor, got {targets}"


# ---------------------------------------------------------------- chaining


def test_chaining_targets_resolve():
    docs = request_docs()
    for name, (rel, doc) in docs.items():
        for code in scripts_of(doc, "after-response"):
            for target in chained_targets(code):
                assert target in docs, f"{rel}: setNextRequest({target!r}) does not exist"


def test_expected_chain_order():
    docs = request_docs()
    for name, expected_next in EXPECTED_CHAIN.items():
        assert name in docs, f"chain start {name!r} missing"
        rel, doc = docs[name]
        targets = [t for code in scripts_of(doc, "after-response") for t in chained_targets(code)]
        actual = targets[0] if targets else None
        assert actual == expected_next, (
            f"{rel}: chain mismatch — expected next {expected_next!r}, got {actual!r}"
        )


def test_terminal_requests_stop_runner():
    for name in ("Sign Out", "Switch Active Role"):
        rel, doc = request_docs()[name]
        codes = scripts_of(doc, "after-response")
        if codes:
            assert "setNextRequest(null)" in " ".join(codes), (
                f"{rel}: terminal request must call setNextRequest(null)"
            )


# ---------------------------------------------------------------- role switching


ROLE_SWITCH_REQUIRED = (
    [f"Patient: {n}" for n in [
        "Get Profile", "List Research Studies", "Record Study Consent",
        "Get Research Progress", "List Questionnaires", "Get Completion Total",
        "Get Questionnaire Detail", "Submit Questionnaire Response",
        "List Record Summary", "Create Journal", "Update Journal",
        "List Upcoming Appointments", "Check Practitioner Availability",
        "Book Appointment", "Update Appointment Status", "Write Referral",
        "Get Circle Stats",
    ]]
    + [f"Practitioner: {n}" for n in [
        "Get Profile", "Get Roles", "Get Schedule", "List Practitioner Slots",
        "Get Busy Slots", "List Sessions", "Get Today Sessions",
        "Update Availability", "Create SOAP Notes",
    ]]
    + ["Cleanup: Delete Practitioner Profile", "Cleanup: Delete Patient Profile"]
)

NO_ROLE_SWITCH = (
    [f"Seed {n}" for n in [
        "Organization", "Location", "Healthcare Service", "Practitioner Role",
        "Schedule", "Questionnaire", "SOAP Questionnaire", "Plan Definition",
        "Research Study",
    ]]
    + ["Admin: Create Questionnaire Draft", "Admin: Update Questionnaire",
       "Cleanup: Delete Organization"]
)


def test_journey_requests_switch_role_in_pre_request():
    docs = request_docs()
    for name in ROLE_SWITCH_REQUIRED:
        rel, doc = docs[name]
        codes = scripts_of(doc, "before-request")
        assert codes, f"{rel}: journey request must have a before-request script"
        joined = "\n".join(codes)
        assert 'bru.setVar("activeRole"' in joined, f"{rel}: before-request must set activeRole"
        assert "bru.sendRequest" in joined, f"{rel}: before-request must switch role via sendRequest"
        assert "/api/v1/auth/active-role" in joined, (
            f"{rel}: before-request must call the active-role endpoint"
        )


def test_superadmin_requests_skip_role_switch():
    docs = request_docs()
    for name in NO_ROLE_SWITCH:
        rel, doc = docs[name]
        codes = scripts_of(doc, "before-request")
        joined = "\n".join(codes)
        assert "runNext" not in joined, f"{rel}: superadmin request must not runNext"


# ---------------------------------------------------------------- superadmin auth


def test_superadmin_requests_use_api_key():
    docs = request_docs()
    for name in NO_ROLE_SWITCH:
        rel, doc = docs[name]
        headers = headers_of(doc)
        assert "X-API-Key" in headers, f"{rel}: superadmin request needs X-API-Key header"
        assert "{{process.env.SUPERADMIN_API_KEY}}" in headers["X-API-Key"], (
            f"{rel}: X-API-Key must use SUPERADMIN_API_KEY env var"
        )


def test_journey_requests_inherit_bearer_without_api_key():
    docs = request_docs()
    for name in ROLE_SWITCH_REQUIRED:
        rel, doc = docs[name]
        headers = headers_of(doc)
        assert "X-API-Key" not in headers, f"{rel}: journey request must not send X-API-Key"
        assert headers.get("Authorization") == "Bearer {{sessionToken}}", (
            f"{rel}: journey request must send an explicit Authorization bearer header"
        )


def test_reference_requests_send_bearer():
    docs = request_docs()
    for name in ["List All Clinic Locations", "List Practitioners in a Clinic",
                 "View Healthcare Service Provided by a Practitioner in a Clinic"]:
        rel, doc = docs[name]
        headers = headers_of(doc)
        assert headers.get("Authorization") == "Bearer {{sessionToken}}", (
            f"{rel}: reference request must send an explicit Authorization bearer header"
        )


def test_folder_yml_has_no_auth_block():
    doc = load(os.path.join(FHIR_DIR, "folder.yml"))
    request_cfg = doc.get("request") or {}
    assert "auth" not in request_cfg, "fhir/folder.yml must not carry folder auth"


# ---------------------------------------------------------------- auth plumbing


def test_active_role_uses_variable():
    doc = load(os.path.join(API_DIR, "auth", "active-role.yml"))
    assert '{{activeRole}}' in body_of(doc).get("data", ""), (
        "active-role.yml body must use {{activeRole}} variable"
    )


def test_switch_active_role_matches_active_role():
    active = load(os.path.join(API_DIR, "auth", "active-role.yml"))
    switch = load(os.path.join(API_DIR, "auth", "switch-active-role.yml"))
    assert active["http"]["url"] == switch["http"]["url"], "switch-active-role must hit the same endpoint"
    assert active["http"]["method"] == switch["http"]["method"]


def test_opencollection_variables():
    doc = load(os.path.join(API_DIR, "opencollection.yml"))
    vars_ = [(v.get("name") or "") for v in (doc.get("request") or {}).get("variables") or []]
    for var in REQUIRED_OPENCOLLECTION_VARS:
        assert var in vars_, f"opencollection.yml is missing variable {var!r}"


# ---------------------------------------------------------------- seed bodies


def test_seed_questionnaire_active_and_regular():
    doc = load(os.path.join(FHIR_DIR, "seed", "seed-questionnaire.yml"))
    data = body_of(doc).get("data", "")
    assert '"status": "active"' in data or '"status":"active"' in data, "seed questionnaire must be active"
    assert "assessment-context" in data, "seed questionnaire must carry the assessment-context useContext"
    assert '"code": "regular"' in data or '"code":"regular"' in data, "seed questionnaire useContext must be regular"


def test_seed_soap_questionnaire_fixed_id():
    doc = load(os.path.join(FHIR_DIR, "seed", "seed-soap-questionnaire.yml"))
    assert '"id": "soap"' in body_of(doc).get("data", ""), "SOAP questionnaire must be seeded with id=soap"
    assert doc["http"]["method"] == "PUT", "SOAP questionnaire must use PUT (Blaze ignores ids on POST)"
    assert doc["http"]["url"].endswith("/fhir/Questionnaire/soap"), "SOAP questionnaire URL must target /fhir/Questionnaire/soap"


def test_seed_research_study_active():
    doc = load(os.path.join(FHIR_DIR, "seed", "seed-research-study.yml"))
    data = body_of(doc).get("data", "")
    assert '"status": "active"' in data or '"status":"active"' in data, "research study must be active"


# ---------------------------------------------------------------- journey bodies


def test_book_appointment_transaction_bundle():
    doc = load(os.path.join(FHIR_DIR, "patient", "book-appointment.yml"))
    data = body_of(doc).get("data", "")
    assert '"resourceType": "Bundle"' in data or '"resourceType":"Bundle"' in data, "booking must be a transaction bundle"
    assert '"Slot"' in data and '"Appointment"' in data, "booking bundle must contain Slot + Appointment"
    assert "busy-tentative" in data, "booking Slot must be status busy-tentative"


def test_write_referral_if_none_match():
    doc = load(os.path.join(FHIR_DIR, "patient", "write-referral.yml"))
    headers = headers_of(doc)
    assert "If-None-Match" in headers and headers["If-None-Match"] == "*", (
        "write-referral must send If-None-Match: *"
    )


def test_check_availability_docs_explain_dynamic_model():
    doc = load(os.path.join(FHIR_DIR, "patient", "check-practitioner-availability.yml"))
    docs = str(doc.get("docs", ""))
    assert "availableTime" in docs, "availability docs must explain the dynamic model (availableTime)"
    assert "busy" in docs, "availability docs must explain busy status subtraction"


def test_every_subfolder_folder_yml_present():
    for folder in EXPECTED_FHIR_TREE:
        doc = load(os.path.join(FHIR_DIR, folder, "folder.yml"))
        assert doc["info"]["type"] == "folder"
