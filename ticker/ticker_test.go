package ticker

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestNewTicker(t *testing.T) {
	typ := SECOND
	delay := 20 * time.Second
	ticker := NewTicker(typ, delay)

	require.Equal(t, typ, ticker.typ)
	require.Equal(t, delay, ticker.delay)
}

func TestTimeNoSecondFractionsCalcWaitTime(t *testing.T) {
	layout := "15:04:05.000"
	timeTest, _ := time.ParseInLocation(layout, "15:04:05.400", time.Local)
	timeExpected, _ := time.ParseInLocation(layout, "15:04:05.000", time.Local)

	timeResult := TimeNoSecondFractions(timeTest)
	require.Equal(t, timeResult, timeExpected)

	timeTest, _ = time.ParseInLocation(layout, "15:04:05.990", time.Local)
	timeExpected, _ = time.ParseInLocation(layout, "15:04:05.000", time.Local)

	timeResult = TimeNoSecondFractions(timeTest)
	require.Equal(t, timeResult, timeExpected)
}

func TestTimeNoMinuteFractions(t *testing.T) {
	layout := "15:04:05.000"

	timeTest, _ := time.ParseInLocation(layout, "15:04:05.400", time.Local)
	timeExpected, _ := time.ParseInLocation(layout, "15:04:00.000", time.Local)

	timeResult := TimeNoMinuteFractions(timeTest)
	require.Equal(t, timeResult, timeExpected)
}

func TestTimeNoHourFractions(t *testing.T) {
	layout := "15:04:05.000"

	timeTest, _ := time.ParseInLocation(layout, "15:04:05.400", time.Local)
	timeExpected, _ := time.ParseInLocation(layout, "15:00:00.000", time.Local)

	timeResult := TimeNoHourFractions(timeTest)
	require.Equal(t, timeResult, timeExpected)
}
