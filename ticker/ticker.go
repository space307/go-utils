// Copyright 2016 Aleksandr Demakin. All rights reserved.

package ticker

import "time"

// TickerType is the type of the ticker. SECOND, MINUTE, or HOUR
type TickerType int

const (
	// SECOND ticker type means that ticks happen at the beginning of every second
	SECOND TickerType = 0
	// MINUTE ticker type means that ticks happen at the beginning of every minute
	MINUTE TickerType = 1
	// HOUR ticker type means that ticks happen at the beginning of every hour
	HOUR TickerType = 2
	// INTERVAL ticker type means that ticks happen every given interval in time.Duration
	INTERVAL TickerType = 3
)

// TimeTicker sends time into a channel every second, minute, or hour at the specified moment
type TimeTicker struct {
	C        chan time.Time
	delay    time.Duration
	stopChan chan struct{}
	typ      TickerType
}

// NewTicker creates a new ticker, which sends current time to its channel
// every second, minute, or hour after the specified delay
func NewTicker(typ TickerType, delay time.Duration) *TimeTicker {
	result := &TimeTicker{
		C:        make(chan time.Time, 1),
		delay:    delay,
		typ:      typ,
		stopChan: make(chan struct{}, 1),
	}
	go result.tick()
	return result
}

// Stop stops the ticker. It is not usable anymore
func (ticker *TimeTicker) Stop() {
	ticker.stopChan <- struct{}{}
}

func (ticker *TimeTicker) calcWaitTime() time.Duration {
	var next time.Time
	now := time.Now()
	switch ticker.typ {
	case SECOND:
		secondStart := TimeNoSecondFractions(now)
		next = secondStart.Add(time.Second + ticker.delay)
	case MINUTE:
		minuteStart := TimeNoMinuteFractions(now)
		next = minuteStart.Add(time.Minute + ticker.delay)
	case HOUR:
		hourStart := TimeNoHourFractions(now)
		next = hourStart.Add(time.Hour + ticker.delay)
	case INTERVAL:
		next = now.Add(ticker.delay)
	}
	return next.Sub(now)
}

func (ticker *TimeTicker) tick() {
	for {
		select {
		case <-ticker.stopChan:
			close(ticker.C)
			return
		case t := <-time.After(ticker.calcWaitTime()):
			// do not block on send. if the receiver cannot handle the tick, it will be missed
			select {
			case ticker.C <- t:
			default:
			}
		}
	}
}

// TimeNoSecondFractions returns the time with all time units
// less, then a second set to 0.
// ex. 11:22:33.456 --> 11:22:33.000
func TimeNoSecondFractions(t time.Time) time.Time {
	return time.Unix(t.Unix(), 0)
}

// TimeNoMinuteFractions returns the time with all time units
// less, then an hour set to 0.
// ex. 11:22:33.456 --> 11:22:00.000
func TimeNoMinuteFractions(t time.Time) time.Time {
	t = TimeNoSecondFractions(t)
	return t.Add(-time.Second * time.Duration(t.Second()))
}

// TimeNoHourFractions returns the time with all time units
// less, then an hour set to 0.
// ex. 11:22:33.456 --> 11:00:00.000
func TimeNoHourFractions(t time.Time) time.Time {
	t = TimeNoMinuteFractions(t)
	return t.Add(-time.Minute * time.Duration(t.Minute()))
}
