package series

import (
	"math"
	"time"
)

const (
	_ = iota
	// ConsolidateAverage represents an average consolidation type.
	ConsolidateAverage
	// ConsolidateFirst represents a first value consolidation type.
	ConsolidateFirst
	// ConsolidateLast represents a last value consolidation type.
	ConsolidateLast
	// ConsolidateMax represents a maximal value consolidation type.
	ConsolidateMax
	// ConsolidateMin represents a minimal value consolidation type.
	ConsolidateMin
	// ConsolidateSum represents a sum consolidation type.
	ConsolidateSum
)

const (
	// OperatorNone represents a null operation type.
	OperatorNone = iota
	// OperatorAverage represents an average operation type.
	OperatorAverage
	// OperatorSum represents a sum operation type.
	OperatorSum
)

type bucket struct {
	startTime time.Time
	points    []Point
}

// Consolidate consolidates points buckets based on consolidation function.
func (b bucket) Consolidate(consolidation int) Point {
	point := Point{
		Value: Value(math.NaN()),
		Time:  b.startTime,
	}

	length := len(b.points)
	if length == 0 {
		return point
	}

	switch consolidation {
	case ConsolidateAverage:
		sum := 0.0
		sumCount := 0
		for _, p := range b.points {
			if p.Value.IsNaN() {
				continue
			}

			sum += float64(p.Value)
			sumCount++
		}

		if sumCount > 0 {
			point.Value = Value(sum / float64(sumCount))
		}

		if length == 1 {
			point.Time = b.points[0].Time
		} else {
			// Interpolate median time
			point.Time = b.points[0].Time.Add(b.points[length-1].Time.Sub(b.points[0].Time) / 2)
		}

	case ConsolidateSum:
		sum := 0.0
		sumCount := 0
		for _, p := range b.points {
			if p.Value.IsNaN() {
				continue
			}

			sum += float64(p.Value)
			sumCount++
		}

		if sumCount > 0 {
			point.Value = Value(sum)
		}

		point.Time = b.points[length-1].Time

	case ConsolidateFirst:
		point = b.points[0]

	case ConsolidateLast:
		point = b.points[length-1]

	case ConsolidateMax:
		for _, p := range b.points {
			if !p.Value.IsNaN() && p.Value > point.Value || point.Value.IsNaN() {
				point = p
			}
		}

	case ConsolidateMin:
		for _, p := range b.points {
			if !p.Value.IsNaN() && p.Value < point.Value || point.Value.IsNaN() {
				point = p
			}
		}
	}

	return point
}

// Normalize aligns multiple point series on a common time step, consolidates points samples if necessary.
func Normalize(series []Series, startTime, endTime time.Time, sample int, consolidation int,
	interpolate bool) ([]Series, error) {

	if sample <= 0 {
		return nil, ErrInvalidSample
	}

	length := len(series)
	if length == 0 {
		return nil, ErrEmptySeries
	}

	result := make([]Series, length)
	buckets := make([][]bucket, length)

	// Calculate the common step for all series based on time range and requested sampling
	step := endTime.Sub(startTime) / time.Duration(sample)

	// Dispatch points into proper time step buckets and then apply consolidation function
	for i, s := range series {
		if s.Points == nil {
			continue
		}

		buckets[i] = make([]bucket, sample)

		// Initialize time steps
		for j := 0; j < sample; j++ {
			buckets[i][j] = bucket{
				startTime: startTime.Add(time.Duration(j) * step),
				points:    make([]Point, 0),
			}
		}

		for _, p := range s.Points {
			// Discard series points out of time specs range
			if p.Time.Before(startTime) || p.Time.After(endTime) {
				continue
			}

			// Stop if index goes beyond the requested sample
			idx := int64(float64(p.Time.UnixNano()-startTime.UnixNano())/float64(step.Nanoseconds())+1) - 1
			if idx >= int64(sample) {
				continue
			}

			buckets[i][idx].points = append(buckets[i][idx].points, p)
		}

		result[i] = Series{
			Points:  make([]Point, sample),
			Summary: make(map[string]Value),
		}

		// Consolidate point buckets
		lastKnown := -1

		for j := range buckets[i] {
			result[i].Points[j] = buckets[i][j].Consolidate(consolidation)

			if interpolate {
				// Keep reference of last and next known points
				if lastKnown != -1 {
					result[i].Points[j].prev = &result[i].Points[lastKnown]
				}

				if !result[i].Points[j].Value.IsNaN() {
					if lastKnown != -1 {
						for k := lastKnown; k < j; k++ {
							result[i].Points[k].next = &result[i].Points[j]
						}
					}

					lastKnown = j
				}
			}

			// Align consolidated points timestamps among normalized series lists
			result[i].Points[j].Time = buckets[i][j].startTime.Add(time.Duration(step.Seconds() * float64(j))).
				Round(time.Second)
		}

		// Interpolate missing points
		if !interpolate {
			continue
		}

		for j, point := range result[i].Points {
			if !point.Value.IsNaN() || point.Value.IsNaN() && (point.prev == nil || point.next == nil) {
				continue
			}

			a := float64(point.next.Value-point.prev.Value) / float64(point.next.Time.UnixNano()-
				point.prev.Time.UnixNano())
			b := float64(point.prev.Value) - a*float64(point.Time.UnixNano())

			result[i].Points[j].Value = Value(a*float64(point.next.Time.UnixNano()) + b)
		}
	}

	return result, nil
}

// Average returns a new series averaging each datapoints.
func Average(series []Series) (Series, error) {
	return applyOperator(series, OperatorAverage)
}

// Sum returns a new series summing each datapoints.
func Sum(series []Series) (Series, error) {
	return applyOperator(series, OperatorSum)
}

func applyOperator(series []Series, operator int) (Series, error) {
	length := len(series)
	if length == 0 {
		return Series{}, ErrEmptySeries
	}

	count := len(series[0].Points)

	result := Series{
		Points:  make([]Point, count),
		Summary: make(map[string]Value),
	}

	for i := 0; i < count; i++ {
		sumCount := 0

		result.Points[i].Time = series[0].Points[i].Time

		for _, s := range series {
			if len(s.Points) != count {
				return Series{}, ErrUnnormalizedSeries
			} else if s.Points[i].Value.IsNaN() {
				continue
			}

			result.Points[i].Value += s.Points[i].Value
			sumCount++
		}

		if sumCount == 0 {
			result.Points[i].Value = Value(math.NaN())
		} else if operator == OperatorAverage {
			result.Points[i].Value /= Value(sumCount)
		}
	}

	return result, nil
}
