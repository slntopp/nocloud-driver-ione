package utils

import (
	ipb "github.com/slntopp/nocloud-proto/instances"
	"github.com/slntopp/nocloud/pkg/nocloud/periods"
	"time"
)

type IClock interface {
	Now() time.Time
}

type Clock struct{}

func (Clock) Now() time.Time { return time.Now() }

func AlignPaymentDate(start int64, end int64, period int64, inst *ipb.Instance) int64 {
	// Apply only on month period
	if period != 30*86400 {
		return end
	}
	// If instance has start date, then apply billing month
	if start <= end && (inst != nil && inst.GetMeta() != nil && inst.GetMeta().Started > 0) {
		return periods.GetNextDate(start, periods.BillingMonth, inst.GetMeta().Started)
	}

	daysInMonth := func(year int, month time.Month) int {
		return time.Date(year, month+1, 0, 0, 0, 0, 0, time.UTC).Day()
	}

	var sign int
	if start <= end {
		sign = 1
	} else {
		sign = -1
	}

	startTime := time.Unix(start, 0).In(time.UTC)
	dayStart := startTime.Day()
	daysInMonthStart := daysInMonth(startTime.Year(), startTime.Month())
	endTime := time.Unix(end, 0).In(time.UTC)

	var delta int
	if sign == 1 {
		delta = daysInMonthStart - startTime.Day() + 1
	} else {
		delta = -(startTime.Day() + 1)
	}
	yearAfterStartDate := startTime.AddDate(0, 0, delta).Year()
	monthAfterStartDate := startTime.AddDate(0, 0, delta).Month()
	daysInMonthEnd := daysInMonth(yearAfterStartDate, monthAfterStartDate)

	// Happens when start is 1st day in 31day month
	if startTime.Month() == endTime.Month() {
		if sign == 1 {
			return startTime.AddDate(0, 1*sign, 0).Unix()
		}
	}

	// Default case, just add month
	if dayStart <= daysInMonthEnd {
		return startTime.AddDate(0, 1*sign, 0).Unix()
	}

	// Overlapping case. Add month and subtract days
	return startTime.AddDate(0, 1*sign, daysInMonthEnd-dayStart).Unix()
}
