package usage

import "time"

func Normalize(snapshot Snapshot, source string, capturedAt time.Time) Record {
	record := Record{CapturedAt: capturedAt, Source: source}
	if snapshot.PlanType != nil {
		record.PlanType = *snapshot.PlanType
	}
	if snapshot.Primary != nil {
		record.FiveHour = normalizeWindow(*snapshot.Primary)
	}
	if snapshot.Secondary != nil {
		record.Weekly = normalizeWindow(*snapshot.Secondary)
	}
	return record
}

func normalizeWindow(window LimitWindow) Window {
	used := window.UsedPercent
	if used == 0 && window.UsedPercentCamel != 0 {
		used = window.UsedPercentCamel
	}
	if used < 0 {
		used = 0
	}
	if used > 100 {
		used = 100
	}
	resetsAt := window.ResetsAt
	if resetsAt == nil {
		resetsAt = window.ResetsAtCamel
	}
	return Window{
		UsedPercent: used,
		LeftPercent: 100 - used,
		ResetsAt:    resetsAt,
	}
}
