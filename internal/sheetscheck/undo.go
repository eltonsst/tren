package sheetscheck

import (
	"context"
	"fmt"

	"google.golang.org/api/sheets/v4"
)

type UndoResult struct {
	SlotID            string
	ExerciseID        string
	TelegramMessageID int
	RemovedSets       int
}

type rowSpan struct {
	start int64
	end   int64
}

// UndoLastExercise deletes every set appended by the most recent Telegram
// result message in an open session. A result message represents one exercise.
func (c *Checker) UndoLastExercise(ctx context.Context, sessionID string) (*UndoResult, error) {
	if sessionID == "" {
		return nil, fmt.Errorf("session ID is required")
	}
	values, err := c.readValues(ctx, "Sets")
	if err != nil {
		return nil, err
	}
	result, spans, err := planUndoLastExercise(values, sessionID)
	if err != nil || result == nil {
		return result, err
	}
	sheetIDs, err := c.sheetIDs(ctx, "Sets")
	if err != nil {
		return nil, err
	}
	requests := make([]*sheets.Request, 0, len(spans))
	for index := len(spans) - 1; index >= 0; index-- {
		span := spans[index]
		requests = append(requests, &sheets.Request{DeleteDimension: &sheets.DeleteDimensionRequest{
			Range: &sheets.DimensionRange{
				SheetId: sheetIDs["Sets"], Dimension: "ROWS",
				StartIndex: span.start, EndIndex: span.end,
			},
		}})
	}
	if _, err := c.service.Spreadsheets.BatchUpdate(
		c.spreadsheetID,
		&sheets.BatchUpdateSpreadsheetRequest{Requests: requests},
	).Context(ctx).Do(); err != nil {
		return nil, fmt.Errorf("delete last exercise sets: %w", err)
	}
	return result, nil
}

func planUndoLastExercise(values [][]any, sessionID string) (*UndoResult, []rowSpan, error) {
	if len(values) == 0 {
		return nil, nil, fmt.Errorf("sheet Sets has no header row")
	}
	sets, err := newTable("Sets", values)
	if err != nil {
		return nil, nil, err
	}
	if err := sets.require("session_id", "slot_id", "exercise_id", "telegram_message_id"); err != nil {
		return nil, nil, err
	}

	messageID := 0
	for index := len(values) - 1; index >= 1; index-- {
		if rawValue(values[index], sets.index, "session_id") != sessionID {
			continue
		}
		messageID, err = parseInteger(rawValue(values[index], sets.index, "telegram_message_id"))
		if err != nil {
			return nil, nil, fmt.Errorf("Sets telegram_message_id: %w", err)
		}
		break
	}
	if messageID == 0 {
		return nil, nil, nil
	}

	result := &UndoResult{TelegramMessageID: messageID}
	var rows []int64
	for index := 1; index < len(values); index++ {
		row := values[index]
		if rawValue(row, sets.index, "session_id") != sessionID {
			continue
		}
		candidate, parseErr := parseInteger(rawValue(row, sets.index, "telegram_message_id"))
		if parseErr != nil {
			return nil, nil, fmt.Errorf("Sets telegram_message_id: %w", parseErr)
		}
		if candidate != messageID {
			continue
		}
		slotID := rawValue(row, sets.index, "slot_id")
		exerciseID := rawValue(row, sets.index, "exercise_id")
		if result.RemovedSets > 0 && (slotID != result.SlotID || exerciseID != result.ExerciseID) {
			return nil, nil, fmt.Errorf("Telegram message %d contains sets for multiple exercises", messageID)
		}
		result.SlotID = slotID
		result.ExerciseID = exerciseID
		result.RemovedSets++
		// The values slice index is also the zero-based Google Sheets row index.
		rows = append(rows, int64(index))
	}

	spans := make([]rowSpan, 0, len(rows))
	for _, row := range rows {
		if len(spans) == 0 || spans[len(spans)-1].end != row {
			spans = append(spans, rowSpan{start: row, end: row + 1})
			continue
		}
		spans[len(spans)-1].end = row + 1
	}
	return result, spans, nil
}
