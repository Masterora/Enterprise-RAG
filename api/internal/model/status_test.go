package model

import "testing"

func TestDocumentStateMachineRejectsBackwardAndPostDeleteTransitions(t *testing.T) {
	previous, ok := DocumentPreviousStatuses(DocumentStatusParsed)
	if !ok || len(previous) == 0 || previous[0] != DocumentStatusParsing {
		t.Fatalf("unexpected parsed transition: %#v", previous)
	}
	for _, status := range previous {
		if status == DocumentStatusDeleting || status == DocumentStatusIndexed {
			t.Fatalf("parsed must not overwrite terminal or later state: %s", status)
		}
	}
}

func TestTaskStateMachineOnlyCompletesRunningTask(t *testing.T) {
	previous, ok := TaskPreviousStatuses(TaskStatusSuccess)
	if !ok || len(previous) != 1 || previous[0] != TaskStatusRunning {
		t.Fatalf("unexpected success transition: %#v", previous)
	}
}

func TestDocumentRetryTransitionsAreSeparatedFromForwardTransitions(t *testing.T) {
	forward, _ := DocumentPreviousStatuses(DocumentStatusParsed)
	retry, _ := DocumentRetryPreviousStatuses(DocumentStatusParsed)
	if len(forward) != 1 || forward[0] != DocumentStatusParsing {
		t.Fatalf("unexpected forward transition: %#v", forward)
	}
	if len(retry) != 2 || retry[0] != DocumentStatusFailed || retry[1] != DocumentStatusChunking {
		t.Fatalf("unexpected retry transition: %#v", retry)
	}
}
