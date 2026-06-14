package system

import "time"

func fixedClock(value time.Time) Clock {
	return func() time.Time {
		return value
	}
}
