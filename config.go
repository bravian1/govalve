package govalve

import (
	"golang.org/x/time/rate"
)

type Option func(*limiterConfig) error

type limiterConfig struct {
	workerPoolSize  int
	queueSize       int
	rateLimiterSize *rate.Limiter

	isDedicated bool
	sharedKey string
}

