package idgen

import "time"

type stepPolicy struct {
	dynamic        bool
	targetDuration time.Duration
}

func (p stepPolicy) next(current int64, minStep int64, maxStep int64, elapsed time.Duration) int64 {
	if current <= 0 {
		current = minStep
	}
	if minStep <= 0 {
		minStep = current
	}
	if maxStep <= 0 {
		maxStep = current
	}
	if current < minStep {
		current = minStep
	}
	if current > maxStep {
		current = maxStep
	}
	if !p.dynamic || p.targetDuration <= 0 || elapsed <= 0 {
		return current
	}
	if elapsed < p.targetDuration {
		next := current * 2
		if next < current || next > maxStep {
			return maxStep
		}
		return next
	}
	if elapsed >= 2*p.targetDuration {
		next := current / 2
		if next < minStep {
			return minStep
		}
		return next
	}
	return current
}
