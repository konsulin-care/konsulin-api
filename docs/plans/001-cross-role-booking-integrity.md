# Cross-Role Booking Integrity — Practitioner-Scoped Day Lock + Overlap Guard

Status: approved (option B)
Owner: backend

## Problem

1. **Reported 409 (bug)**: `AcquireLocksForAppointment` (slot_usecase.go:1380) resolves the timezone of **every** PractitionerRole of the practitioner before booking. Schedule-less / period-less roles (e.g. the `researcher` role `DH73BW3KCNK6KBME`) abort the whole booking with `cannot determine timezone: invalid period.start and period.end`, mapped to a misleading 409.
2. **Business rule**: one practitioner may practice at multiple locations → multiple bookable PractitionerRoles, each with its own Schedule. The frontend generates available times as *plan minus non-free slots* per role, so a time booked via role A must not be offered via role B.
3. **Current locks don't enforce the rule**: day-lock keys are per `(scheduleID, local-day, tz)`; concurrent bookings via different roles of the same practitioner never contend → cross-role overbooking is possible. The "lock all roles" fan-out is a phantom guarantee (per-schedule keys) and is the source of bug #1.

## Chosen approach (option B)

1. **Practitioner-scoped day lock** — single key family for **all** slot mutators:
   `slotgen:lock:practitioner:<practitionerID>:<YYYY-MM-DD>`
   One practitioner ⇒ at most one booking per overlapping window ⇒ the practitioner-day is the natural serialization unit.
2. **Booking** locks the practitioner's affected days (computed from the **booked role's** timezone — sibling timezones are never resolved), then runs a **cross-role overlap check**: for every sibling schedule, query slots with status `busy-unavailable,busy-tentative` overlapping `[start, end)` → 409 if any. Race-free because every concurrent booking of this practitioner contends on the same practitioner-day keys.
3. **Callbacks unchanged semantically**: paid → booked slot `busy-unavailable`; expired → booked slot deleted. Nothing to track, nothing to restore.
4. **Availability read path**: new endpoint exposing a practitioner's blocked intervals for a date across all schedulable roles, so the frontend excludes them without touching sibling slot statuses.

## Design decisions

- Lock key drops the `tzName` suffix. Each caller computes local days in its own context: booking/callback use the slot's offset; worker/on-demand/set-unavailability use the role's `Period` tz. Document the same-timezone-across-roles assumption (roles in different tz may compute different day sets for the same instant — accepted, rare, only affects lock contention, not conflict correctness which compares instants).
- Booking **never resolves sibling timezones** → bug #1 class eliminated. Schedulable-role filtering: roles without a schedule are skipped entirely (no lock, no tz resolution, no overlap check).
- `AcquireLocksForSlot` / `AcquireLocksForAppointment` are replaced by a single `AcquireLocksForPractitionerDay(ctx, practitionerID, start, end, ttl)`; the callback resolves practitionerID from the role it already references (`fields.PractitionerRoleID`).
- Availability check is by **interval overlap**, not exact-time match — handles differing slot durations across locations.

## Tasks (atomic, TDD)

### Task 1 — Practitioner-day lock primitives (slot_usecase.go)
- Replace `dayLockTarget` with `practitionerDayLockTarget{PractitionerID string; Day time.Time}`.
- `dayTargetsForWindow` → `practitionerDayTargetsForWindow(practitionerID, loc, start, end)` (keep midnight-exclusive end semantics).
- `dayLockKey` → `practitionerDayLockKey(practitionerID, day)` → `slotgen:lock:practitioner:<id>:<YYYY-MM-DD>`.
- `tryAcquireDayLock` → `tryAcquirePractitionerDayLock`; update `acquireDayLocksOrdered`, `dedupeDayTargets`, `sortDayTargets` (key: practitionerID + day).
- **DoD**: unit tests — key format; day boundaries (end at midnight excludes next day); dedupe across schedules of same practitioner; deterministic sort; no other call sites reference old primitives.

### Task 2 — Worker, on-demand regeneration, set-unavailability switch
- `HandleAutomatedSlotGeneration` (worker): practitionerID from `role.Practitioner.Reference`; acquire practitioner-day lock per day.
- `HandleOnDemandSlotRegeneration`: practitioner-day targets; practitionerID from the resolved role context.
- `resolveAndLockWindows` (set-unavailability): practitioner-day targets; practitionerID from first role (input is one practitioner; ownership already validated); days computed per role in its own tz, unioned.
- **DoD**: unit tests assert practitioner-day keys used; worker still skips period-less roles (existing behavior); regression: schedule-less sibling role no longer breaks set-unavailability/worker paths.

### Task 3 — Booking: booked-role-scoped lock + cross-role overlap check
- `contracts/slot.go`: replace `AcquireLocksForAppointment` and `AcquireLocksForSlot` with `AcquireLocksForPractitionerDay(ctx, practitionerID string, start, end time.Time, ttl time.Duration) (func(context.Context), error)`. Update mock in `appointment_payment_notification_test.go`.
- `payment_usecase_impl.go` `HandleAppointmentPayment`:
  - practitionerID from `precond.PractitionerRole.Practitioner.Reference`.
  - Acquire practitioner-day locks (days from booked role's tz).
  - After revalidation + booked-schedule overlap check (existing), extend: for each role in `allPractitionerRoles` with a schedule (skip others; booked role included), query non-free slots overlapping the window → 409 `SlotNoLongerAvailableMessage` if any overlap.
- **DoD**: regression test — practitioner with period-less, schedule-less sibling role books successfully (the reported 409); sibling `busy-tentative` in window → 409; sibling `busy-unavailable` → 409; clean siblings → success; locks released on every failure path.

### Task 4 — Callback lock switch
- `handleAppointmentPaymentNotification`: fetch role by `fields.PractitionerRoleID` → practitionerID; acquire practitioner-day locks from slot window; keep existing revalidation and paid/expired dispatch.
- **DoD**: notification tests updated; expired still deletes booked slot; paid still bundles Appointment + `busy-unavailable`; lock key uses practitioner id.

### Task 5 — Availability endpoint (read-side exclusion)
- New `GET` endpoint (schedule router + controller + usecase): blocked intervals for a practitioner on a date, unioned across all schedulable roles (`busy-unavailable`, `busy-tentative`).
- Document the response contract for the frontend (consume: exclude these intervals when generating available times).
- **DoD**: handler/usecase tests; empty result when no blocks; includes sibling-role blocks; `sync_bruno_api_docs` run + YAML filled.

### Task 6 — Docs & verification
- Update docstrings (`contracts/slot.go`, slot_usecase.go comments), `docs/ARCHITECTURE.md` locking section, `docs/KNOWN-PITFALLS.md` note on period-less roles.
- **DoD**: `go build ./...`; `go test ./...` green; `golangci-lint run` clean; Bruno docs in sync; manual verification against live data: re-run the reported booking request → success (or 409 only for genuine slot conflicts).

## Definition of done (overall)

1. The reported 409 is fixed: booking succeeds for a practitioner who has period-less / schedule-less sibling roles (regression test + live verification).
2. Overbooking is prevented: a booking overlapping a non-free interval in **any** of the practitioner's schedulable roles returns 409 (unit tests).
3. All slot mutators (booking, callback, worker, on-demand regen, set-unavailability) use the same `slotgen:lock:practitioner:<id>:<date>` family.
4. Paid/expired callback behavior unchanged (busy-unavailable / delete).
5. Availability endpoint exposes cross-role blocked intervals; frontend contract documented.
6. Full test suite + lint green; Bruno API docs in sync.
