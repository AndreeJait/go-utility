package timew

import (
	"time"
)

type Time interface {
	Now() time.Time
	GetTimeZone() *time.Location
}

type definedTime struct {
	Location *time.Location
}

func (d *definedTime) Now() time.Time {
	timeNow := time.Now().In(d.GetTimeZone())
	return timeNow
}

func (d *definedTime) GetTimeZone() *time.Location {
	return d.Location
}

func LoadLocation(name string) *time.Location {
	loc, err := time.LoadLocation(name)
	if err != nil {
		loc, _ = time.LoadLocation("Asia/Jakarta")
	}

	return loc
}

func New(location *time.Location) Time {
	return &definedTime{
		Location: location,
	}
}
