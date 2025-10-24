package slot

import "time"

// clock holds a local wall time (hour and minute).
type clock struct {
	H int
	M int
}

// dayWindow defines an inclusive start and exclusive end wall-clock window for a single day.
type dayWindow struct {
	Start clock
	End   clock
}

// weeklyPlan lists zero or more windows per weekday.
type weeklyPlan struct {
	Monday    []dayWindow
	Tuesday   []dayWindow
	Wednesday []dayWindow
	Thursday  []dayWindow
	Friday    []dayWindow
	Saturday  []dayWindow
	Sunday    []dayWindow
}

// ForWeekday returns windows for the given weekday.
func (wp weeklyPlan) forWeekday(wd time.Weekday) []dayWindow {
	switch wd {
	case time.Monday:
		return wp.Monday
	case time.Tuesday:
		return wp.Tuesday
	case time.Wednesday:
		return wp.Wednesday
	case time.Thursday:
		return wp.Thursday
	case time.Friday:
		return wp.Friday
	case time.Saturday:
		return wp.Saturday
	case time.Sunday:
		return wp.Sunday
	default:
		return nil
	}
}

// interval is a concrete timestamped slot instance.
type interval struct {
	Start time.Time
	End   time.Time
}
