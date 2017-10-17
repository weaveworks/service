package time

import "fmt"

// Duration represents an amount of time and an associated unit.
// It aims to ease conversions from one unit to another, and present to
// users an invoice with numbers human beings can easily reason about, i.e.
// not too small and not too big.
type Duration interface {
	// GetAmount is the amount of time for this duration.
	GetAmount() uint64
	// Unit is the unit for this duration.
	Unit() string
	// ToSeconds converts this duration to seconds.
	ToSeconds() Duration
	// ToMinutes converts this duration to minutes.
	ToMinutes() Duration
	// ToHours converts this duration to hours.
	ToHours() Duration
	// ToDays converts this duration to days.
	ToDays() Duration
	// ToMostReadableUnit converts this duration to the most readable unit for human beings.
	ToMostReadableUnit() Duration
	// String pretty-prints this duration.
	String() string
}

// The following units should also be added to Zuora.
// To do so (or verify the current state of Zuora's configuration), go to:
// Zuora > <username> > Settings > Billing > Customize Units of Measure
const (
	// NodeSeconds represents the node-seconds unit.
	NodeSeconds string = "node-seconds"
	// NodeMinutes represents the node-minutes unit.
	NodeMinutes string = "node-minutes"
	// NodeHours represents the node-hours unit.
	NodeHours string = "node-hours"
	// NodeDays represents the node-days unit.
	NodeDays string = "node-days"
)

// Seconds is a Duration which represents an amount of seconds.
type Seconds struct {
	Amount uint64
}

// GetAmount is the number of seconds for this duration.
func (s Seconds) GetAmount() uint64 {
	return s.Amount
}

// Unit returns "node-seconds".
func (s Seconds) Unit() string {
	return NodeSeconds
}

// ToSeconds returns this.
func (s Seconds) ToSeconds() Duration {
	return s
}

// ToMinutes returns this amount of seconds converted to minutes.
func (s Seconds) ToMinutes() Duration {
	return Minutes{s.Amount / 60}
}

// ToHours returns this amount of seconds converted to hours.
func (s Seconds) ToHours() Duration {
	return Hours{s.Amount / 60 / 60}
}

// ToDays returns this amount of seconds converted to days.
func (s Seconds) ToDays() Duration {
	return Days{s.Amount / 60 / 60 / 24}
}

// String pretty-prints this duration.
func (s Seconds) String() string {
	return fmt.Sprintf("%v %v", s.GetAmount(), s.Unit())
}

// ToMostReadableUnit returns this amount of seconds converted to the unit generating the smallest possible number.
func (s Seconds) ToMostReadableUnit() Duration {
	if s.Amount < 60 {
		return s
	} else if s.Amount < 60*60 {
		return s.ToMinutes()
	} else if s.Amount < 60*60*24 {
		return s.ToHours()
	}
	return s.ToDays()
}

// Minutes is a Duration which represents an amount of minutes.
type Minutes struct {
	Amount uint64
}

// GetAmount is the number of minutes for this duration.
func (m Minutes) GetAmount() uint64 {
	return m.Amount
}

// Unit returns "node-minutes".
func (m Minutes) Unit() string {
	return NodeMinutes
}

// ToSeconds returns this amount of minutes converted to seconds.
func (m Minutes) ToSeconds() Duration {
	return Seconds{m.Amount * 60}
}

// ToMinutes returns this.
func (m Minutes) ToMinutes() Duration {
	return m
}

// ToHours returns this amount of minutes converted to hours.
func (m Minutes) ToHours() Duration {
	return Hours{m.Amount / 60}
}

// ToDays returns this amount of minutes converted to days.
func (m Minutes) ToDays() Duration {
	return Days{m.Amount / 60 / 24}
}

// ToMostReadableUnit returns this amount of minutes converted to the unit generating the smallest possible number.
func (m Minutes) ToMostReadableUnit() Duration {
	if m.Amount < 60 {
		return m
	} else if m.Amount < 60*24 {
		return m.ToHours()
	}
	return m.ToDays()
}

// String pretty-prints this duration.
func (m Minutes) String() string {
	return fmt.Sprintf("%v %v", m.GetAmount(), m.Unit())
}

// Hours is a Duration which represents an amount of hours.
type Hours struct {
	Amount uint64
}

// GetAmount is the number of hours for this duration.
func (h Hours) GetAmount() uint64 {
	return h.Amount
}

// Unit returns "node-hours".
func (h Hours) Unit() string {
	return NodeHours
}

// ToSeconds returns this amount of hours converted to seconds.
func (h Hours) ToSeconds() Duration {
	return Seconds{h.Amount * 60 * 60}
}

// ToMinutes returns this amount of hours converted to minutes.
func (h Hours) ToMinutes() Duration {
	return Minutes{h.Amount * 60}
}

// ToHours returns this.
func (h Hours) ToHours() Duration {
	return h
}

// ToDays returns this amount of hours converted to days.
func (h Hours) ToDays() Duration {
	return Days{h.Amount / 24}
}

// ToMostReadableUnit returns this amount of hours converted to the unit generating the smallest possible number.
func (h Hours) ToMostReadableUnit() Duration {
	if h.Amount < 24 {
		return h
	}
	return h.ToDays()
}

// String pretty-prints this duration.
func (h Hours) String() string {
	return fmt.Sprintf("%v %v", h.GetAmount(), h.Unit())
}

// Days is a Duration which represents an amount of days.
type Days struct {
	Amount uint64
}

// GetAmount is the number of days for this duration.
func (d Days) GetAmount() uint64 {
	return d.Amount
}

// Unit returns "node-days".
func (d Days) Unit() string {
	return NodeDays
}

// ToSeconds returns this amount of days converted to seconds.
func (d Days) ToSeconds() Duration {
	return Seconds{d.Amount * 60 * 60 * 24}
}

// ToMinutes returns this amount of days converted to minutes.
func (d Days) ToMinutes() Duration {
	return Minutes{d.Amount * 60 * 24}
}

// ToHours returns this amount of days converted to hours.
func (d Days) ToHours() Duration {
	return Hours{d.Amount * 24}
}

// ToDays returns this.
func (d Days) ToDays() Duration {
	return d
}

// ToMostReadableUnit returns this amount of days converted to the unit generating the smallest possible number.
func (d Days) ToMostReadableUnit() Duration {
	return d
}

// String pretty-prints this duration.
func (d Days) String() string {
	return fmt.Sprintf("%v %v", d.GetAmount(), d.Unit())
}
