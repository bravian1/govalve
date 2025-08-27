package govalve

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// Manager is the central component of govalve.
// It manages profiles, subscriptions, and limiters.
// The user of the library creates a Manager and registers profiles with it.
// The Manager is safe for concurrent use.
type Manager struct {
	profiles map[string]*limiterConfig
	limiters sync.Map
	store    Store

	mu     sync.RWMutex
	ctx    context.Context
	cancel context.CancelFunc
}

// ManagerOption is a function that configures a Manager.
// It is used to apply options to the Manager during creation.
// For example, to configure a store for the Manager.
type ManagerOption func(*Manager)

// WithStore is a ManagerOption that configures a store for the Manager.
// The store is used to persist subscription data.
// If no store is configured, subscription features are disabled.
func WithStore(store Store) ManagerOption {
	return func(m *Manager) {
		m.store = store
	}
}

// NewManager creates a new Manager.
// It takes a context and a list of ManagerOptions.
// The context is used to gracefully shut down the Manager and all its limiters.
func NewManager(ctx context.Context, opts ...ManagerOption) *Manager {
	managerCtx, cancel := context.WithCancel(ctx)
	m := &Manager{
		profiles: make(map[string]*limiterConfig),
		ctx:      managerCtx,
		cancel:   cancel,
	}

	for _, opt := range opts {
		opt(m)
	}

	return m
}

// RegisterProfile registers a new profile with the Manager.
// A profile defines the configuration for a limiter, such as the rate limit, worker pool size, and usage quota.
// It is identified by a unique name.
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

// Subscribe creates a new subscription for a user.
// It takes a user ID, a profile name, and a duration.
// It creates a new subscription and saves it to the store.
// It returns the new subscription.
func (m *Manager) Subscribe(ctx context.Context, userID, profileName string, duration time.Duration) (*Subscription, error) {
	if m.store == nil {
		return nil, fmt.Errorf("store is not configured")
	}

	m.mu.RLock()
	_, ok := m.profiles[profileName]
	m.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("profile '%s' not found", profileName)
	}

	sub := &Subscription{
		UserID:      userID,
		ProfileName: profileName,
		Status:      SubscriptionStatusActive,
		StartDate:   time.Now(),
		EndDate:     time.Now().Add(duration),
		Usage:       0,
	}

	if err := m.store.SaveSubscription(ctx, sub); err != nil {
		return nil, fmt.Errorf("failed to save subscription: %w", err)
	}

	return sub, nil
}

// GetSubscription retrieves a subscription for a user from the store.
func (m *Manager) GetSubscription(ctx context.Context, userID string) (*Subscription, error) {
	if m.store == nil {
		return nil, fmt.Errorf("store is not configured")
	}
	return m.store.GetSubscription(ctx, userID)
}

// GetLimiter returns a limiter for a user.
// It first retrieves the user's subscription from the store.
// It checks if the subscription is active and not expired.
// It then returns a limiter based on the subscription's profile.
// If the limiter does not exist, it creates a new one.
func (m *Manager) GetLimiter(ctx context.Context, userID string) (Limiter, error) {
	if m.store == nil {
		return nil, fmt.Errorf("store is not configured - use GetLimiterForProfile for direct profile access")
	}

	sub, err := m.GetSubscription(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get subscription: %w", err)
	}

	if sub.Status != SubscriptionStatusActive {
		return nil, fmt.Errorf("subscription is not active")
	}

	if time.Now().After(sub.EndDate) {
		return nil, fmt.Errorf("subscription has expired")
	}

	return m.getLimiterForProfile(ctx, sub.ProfileName, userID)
}

// GetLimiterForProfile returns a limiter for a user based on a profile name.
// This bypasses subscription checks and is useful for simple use cases without subscription management.
func (m *Manager) GetLimiterForProfile(ctx context.Context, profileName, userID string) (Limiter, error) {
	return m.getLimiterForProfile(ctx, profileName, userID)
}

func (m *Manager) getLimiterForProfile(ctx context.Context, profileName, userID string) (Limiter, error) {
	m.mu.RLock()
	profile, ok := m.profiles[profileName]
	m.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("profile '%s' not found", profileName)
	}

	var limiterKey string
	var newLimiterConfig *limiterConfig

	if profile.isDedicated {
		limiterKey = fmt.Sprintf("dedicated-%s-%s", profileName, userID)
		newLimiterConfig = profile
	} else {
		limiterKey = fmt.Sprintf("shared-%s", profile.sharedKey)
		newLimiterConfig = profile
	}

	if l, ok := m.limiters.Load(limiterKey); ok {
		return l.(Limiter), nil
	}

	l, err := newLimiter(m.ctx, newLimiterConfig, m, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to create new limiter: %w", err)
	}
	actualLimiter, loaded := m.limiters.LoadOrStore(limiterKey, l)
	if loaded {
		l.Close()
	}
	return actualLimiter.(Limiter), nil
}

// Shutdown gracefully shuts down the Manager and all its limiters.
// It cancels the context and waits for all workers to finish.
func (m *Manager) Shutdown() {
	m.cancel()
	m.limiters.Range(func(key, value interface{}) bool {
		if limiter, ok := value.(Limiter); ok {
			limiter.Close()
		}
		return true
	})
}
