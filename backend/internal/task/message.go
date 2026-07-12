package task

type Message struct {
	TaskID         string `json:"task_id"`
	DocID          string `json:"doc_id"`
	ProcessingMode string `json:"processing_mode,omitempty"`
}

type ParseTaskMetadata struct {
	ProcessingMode string `json:"processing_mode,omitempty"`
}

const (
	ProcessingModeStandard = "standard"
	ProcessingModeEnhanced = "enhanced"
)

func NormalizeProcessingMode(mode string) string {
	switch mode {
	case ProcessingModeStandard:
		return ProcessingModeStandard
	case ProcessingModeEnhanced:
		return ProcessingModeEnhanced
	default:
		return ProcessingModeEnhanced
	}
}
