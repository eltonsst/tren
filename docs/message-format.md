# Telegram exercise-result format

The parser is deterministic and uses the current program exercise as context. A message normally represents all working sets for one exercise.

## Normal reply

```text
5@2 5@2 5@1 4@1
```

Each set is `reps@RIR`. Spaces, commas, and semicolons are all accepted between sets. `pain0` to `pain10` (or `p0` to `p10`) applies to the exercise's sets. Missing RIR or pain is accepted with a confirmation warning rather than silently invented.

The bot shows a valid example after `/start`, `/resume`, every accepted exercise, and every parsing error, so the format does not need to be remembered.

Send `/howto` at any time for the complete logging cheat sheet inside Telegram.

## Load override

```text
77.5: 5@2 5@1 4@1 4@1
```

The prefix replaces the suggested load for every set. Decimal commas are accepted in the prefix:

```text
77,5kg: 5@2; 5@1; 4@1; 4@1 p0
```

For weighted pull-ups, `+5:` is accepted and the stored load is the external added load.

## Different loads within an exercise

```text
75x5@2, 75x5@1, 72.5x6@1 pain0
```

## Substitution

The parser reserves `swap` and `sub` syntax, but the Telegram workflow does not enable substitutions yet. Change the planned exercise in `Program` before the rotation locks, or log the planned exercise and correct the Sheet manually. Tren rejects substitution messages instead of guessing or silently recording the wrong exercise.

## Correcting the latest result

Send `/undoexercise` while a workout is open to delete every set created by the latest exercise-result message. Tren confirms the exercise and number of sets removed, then shows the logging prompt for that exercise again.

After `/finish`, send `/undo` to permanently delete the latest completed session and all its sets, then make that workout next again. There must be no open workout. Undoing Workout A also unlocks its program version when no sessions remain. Undoing Workout D removes the newly generated version only while it is still unlocked and unused, then reactivates the previous version.

## Confirmation behavior

The bot saves only after the full message parses. It then repeats the interpreted exercise, load, reps, RIR, pain, and substitution. Set-count mismatches and missing RIR or pain appear as warnings. Older corrections can still be made manually in the spreadsheet.
