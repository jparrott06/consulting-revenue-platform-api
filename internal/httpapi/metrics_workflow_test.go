package httpapi

import "testing"

func TestRecordWorkflowConflict_NoPanic(t *testing.T) {
	recordWorkflowConflict("invoice", "generate")
	recordWorkflowConflict("time_entry", "submit")
}
