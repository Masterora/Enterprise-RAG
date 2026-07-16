package model

type AdminSummary struct {
	SubjectTotal     int64
	DocumentTotal    int64
	ChunkTotal       int64
	SessionTotal     int64
	IndexedTotal     int64
	ProcessingTotal  int64
	FailedTotal      int64
	PendingTaskTotal int64
	RunningTaskTotal int64
	FailedTaskTotal  int64
}
