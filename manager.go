package govalve

import (
	"context"
	"fmt"
	"sync"
)

type Manager struct {
	profiles map[string]*limiterConfig
	limiters sync.Map

	mu     sync.RWMutex
	ctx    context.Context
	cancel context.CancelFunc
}

func NewManager(ctx context.Context) *Manager {
	managerCtx, cancel := context.WithCancel(ctx)
	return &Manager{
		profiles: make(map[string]*limiterConfig),
		ctx:      managerCtx,
		cancel:   cancel,
	}
}

func (m *Manager) RegisterProfile(name string, opts ...Option) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, exists := m.profiles[name]; exists {
		return fmt.Errorf("profile %s already exists", name)
	}
	cfg := &limiterConfig{}
	for _, opt := range opts {
		if err := opt(cfg); err != nil {
			return err
		}
	}

	if !cfg.isDedicated && cfg.sharedKey == "" {
		return fmt.Errorf("profile %s must have a shared key if not dedicated", name)
	}

	m.profiles[name] = cfg
	return nil
}

func (m *Manager) GetLimiter(profileName string, userIdentifier string) (Limiter, error) {
	m.mu.RLock()
	profile, ok := m.profiles[profileName]
	m.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("profile '%s' not found", profileName)
	}

	var limiterKey string
	var newLimiterConfig *limiterConfig

	if profile.isDedicated {
		if userIdentifier == "" {
			return nil, fmt.Errorf("userIdentifier is required for dedicated resource profiles")
		}
		limiterKey = fmt.Sprintf("dedicated-%s-%s", profileName, userIdentifier)
		newLimiterConfig = profile
	} else {
		limiterKey = fmt.Sprintf("shared-%s", profile.sharedKey)
		newLimiterConfig = profile
	}

	if l, ok := m.limiters.Load(limiterKey); ok {
		return l.(Limiter), nil
	}

	l, err := newLimiter(m.ctx, newLimiterConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create new limiter: %w", err)
	}
	actualLimiter, loaded := m.limiters.LoadOrStore(limiterKey, l)
	if loaded {
		l.Close()
	}
	return actualLimiter.(Limiter), nil
}

func (m *Manager) Shutdown() {
	m.cancel()
	m.limiters.Range(func(key, value interface{}) bool {
		if limiter, ok := value.(Limiter); ok {
			limiter.Close()
		}
		return true
	})
}
