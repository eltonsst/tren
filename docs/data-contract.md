# Google Sheets data contract

Tren treats Google Sheets as its only durable source of truth. The bot, dashboard, and CLI must address columns by header name, never by column position, so the user may reorder columns without breaking the application.

The import-ready template is `docs/tren-data-contract.xlsx`.

## Tabs

| Tab | Ownership | Purpose |
| --- | --- | --- |
| `Start Here` | Documentation | Public-template setup checklist and ownership guide; ignored by the application |
| `Config` | User and application | Schema version, timezone, active program version, next workout, and authorized Telegram user |
| `Exercises` | User | Stable exercise identifiers, parser aliases, load conventions, and substitutions |
| `ProgramVersions` | Application, user before lock | One row per A-D rotation with lifecycle timestamps |
| `Program` | User and progression engine | Ordered exercise prescriptions for each program version |
| `Sessions` | Application | One row per started workout |
| `Sets` | Application, manually correctable | One row per completed working set |
| `Measurements` | User | Daily body weight, waist, nutrition habits, activity, sleep, recovery, and shoulder symptoms |
| `Inbox` | Application | Telegram audit trail and update-level idempotency |

## Invariants

- IDs are stable text values. Row numbers are never identifiers.
- All loads and body weights are stored in kilograms.
- `Sets` contains working sets only.
- `load_kg` follows the exercise's `load_mode`: total barbell load, external added load for weighted pull-ups, or per-hand load for dumbbells.
- Missing measurements and optional set fields stay blank; zero is a real recorded value.
- A session advances the rotation only after an explicit `/finish`.
- `/undo` removes the latest completed session and its sets in one Sheets batch transaction, then rewinds `next_workout`; Workout D rollback also removes the unused generated version and reactivates its source version.
- There may be at most one open session.
- A Telegram `update_id` is processed at most once. Repeated updates read the existing `Inbox` result rather than appending duplicate sets.
- Program versions are append-only after `locked_at` is populated. The bot locks a version when its Workout A starts.
- `planned_sets` and `planned_rir_*` describe the current rotation; `target_sets` and `target_rir_*` preserve the normal prescription used when temporary reductions end.
- The automatically generated next version may be edited after activation and before `locked_at` is populated.
- Completed program versions and logged sets are never rewritten by automatic progression.
- Manual corrections to `Sets`, `Sessions`, and `Measurements` are allowed and are picked up on the next read.
- Exercises are retired with `active = false`; they are not deleted if referenced by history.

## Program-version lifecycle

1. Finishing Workout D marks the current version `completed` and records `completed_at`.
2. The progression engine copies its `Program` rows into a new version.
3. Earned load changes are applied to the new rows. Rotation 2 also restores `planned_sets` and `planned_rir_*` to their target values after the reduced first rotation.
4. The new version becomes `active` and `Config.active_program_version` points to it.
5. The user may add, remove, substitute, or reorder exercises before the next Workout A starts.
6. Starting Workout A records `locked_at` atomically with the new session; the version is then treated as immutable. Workouts B-D reject an unlocked version.

The completion, copy, activation, and Config changes are submitted in one Sheets batch transaction. If any requested change is invalid, the rollover is not partially applied.

## Initial seed

The seed represents Block 1, rotation 1:

- Reduced first-rotation set counts and approximately 3 RIR on compounds
- Full volume preserved in `target_sets`
- Original A-D exercise order and provisional starting loads
- Later triceps decision: shoulder-width close-grip push-ups or band pushdowns at home, cable pushdowns at the gym
- Gym row and pulldown substitutions recorded as exercise IDs

## Schema evolution

Any column addition or behavior change that affects readers increments `Config.schema_version`. Migrations must preserve unknown user-added columns and never discard rows.
