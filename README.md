# Govalve: A Go Rate Limiter & Subscription Management Library

Package `govalve` provides a powerful, flexible mechanism for managing concurrency, rate-limiting, and subscriptions for users accessing shared or dedicated resources.

## Overview

Govalve solves the complex challenge of **fair resource allocation** in multi-tenant systems. Whether you're building a SaaS platform, API gateway, or microservices architecture, govalve provides enterprise-grade resource management with subscription-based access control.

### The Problem Govalve Solves

Modern applications face critical resource management challenges:
- **API Rate Limiting**: Prevent abuse while ensuring fair access to third-party APIs
- **Database Connection Pooling**: Manage expensive database connections across user tiers
- **Computational Resource Sharing**: Control access to CPU-intensive operations
- **Multi-Tenant Fairness**: Ensure no single user monopolizes shared resources
- **Subscription Enforcement**: Automatically enforce usage quotas and time-based limits

### Why Choose Govalve?

Unlike simple rate limiters, govalve provides **architectural flexibility** for complex business requirements:

- **Tiered Access Control**: Free users share resources, premium users get dedicated capacity
- **Dynamic Subscription Management**: Real-time quota enforcement with persistent state
- **Zero-Downtime Scaling**: Add/remove capacity without service interruption
- **Production-Ready**: Built for high-concurrency environments with graceful degradation

## Key Features

### 🏗️ **Flexible Architecture**
- **Shared Resources**: Pool users together (e.g., free tier shares 100 req/min)
- **Dedicated Resources**: Isolated capacity per user (e.g., enterprise gets private 1000 req/min)
- **Hybrid Models**: Mix shared and dedicated tiers in the same application

### 💳 **Subscription Management**
- **Time-Based Expiration**: Automatic subscription lifecycle management
- **Usage Quotas**: Enforce monthly/daily limits with atomic tracking
- **Real-Time Validation**: Block expired or over-quota users instantly
- **Persistent State**: Pluggable storage for any database (Redis, PostgreSQL, etc.)

### ⚡ **Performance & Reliability**
- **Worker Pool Management**: Configurable concurrency per tier
- **Backpressure Handling**: Queue management prevents system overload
- **Graceful Shutdown**: Zero-downtime deployments with in-flight task completion
- **Context-Aware**: Full support for request timeouts and cancellation

### 🔧 **Developer Experience**
- **Functional Options**: Clean, extensible configuration API
- **Thread-Safe**: Production-ready concurrent access
- **Comprehensive Examples**: Real-world usage patterns included

## Installation

```bash
go get github.com/bravian1/govalve
```

## Quick Start

The primary entry point to the library is the `Manager`. You create a manager, register configuration profiles for your user tiers, and then manage user subscriptions.

```go
package main

import (
	"context"
	"fmt"
	sync "sync"
	time "time"

	"github.com/bravian1/govalve"
	"golang.org/x/time/rate"
)

// MockStore is an in-memory implementation of the govalve.Store interface for demonstration purposes.
type MockStore struct {
	subscriptions map[string]*govalve.Subscription
	mu            sync.RWMutex
}

// NewMockStore creates a new MockStore.
func NewMockStore() *MockStore {
	return &MockStore{
		subscriptions: make(map[string]*govalve.Subscription),
	}
}

// GetSubscription retrieves a subscription from the in-memory store.
func (s *MockStore) GetSubscription(ctx context.Context, userID string) (*govalve.Subscription, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	sub, ok := s.subscriptions[userID]
	if !ok {
		return nil, fmt.Errorf("subscription not found for user %s", userID)
	}

	return sub, nil
}

// SaveSubscription saves a subscription to the in-memory store.
func (s *MockStore) SaveSubscription(ctx context.Context, sub *govalve.Subscription) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.subscriptions[sub.UserID] = sub
	return nil
}

// CheckAndIncrementUsage checks quota and increments usage atomically.
func (s *MockStore) CheckAndIncrementUsage(ctx context.Context, userID string, quota int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	sub, ok := s.subscriptions[userID]
	if !ok {
		return fmt.Errorf("subscription not found for user %s", userID)
	}
	if quota > 0 && sub.Usage >= quota {
		return fmt.Errorf("usage quota exceeded")
	}
	sub.Usage++
	return nil
}

func main() {
	appCtx, appCancel := context.WithCancel(context.Background())
	defer appCancel()

	// 1. Create a new MockStore.
	store := NewMockStore()

	// 2. Create a new Manager with the MockStore.
	manager := govalve.NewManager(appCtx, govalve.WithStore(store))
	defer manager.Shutdown()

	// 3. Register a "pro-tier" profile with a usage quota.
	err := manager.RegisterProfile("pro-tier",
		govalve.WithDedicatedResource(rate.Limit(10)),
		govalve.WithWorkerPool(10, 200),
		govalve.WithUsageQuota(5),
	)
	if err != nil {
		panic(err)
	}

	// 4. Subscribe a user to the "pro-tier" for a short duration.
	userID := "pro-user-123"
	_, err = manager.Subscribe(appCtx, userID, "pro-tier", 5*time.Second)
	if err != nil {
		panic(err)
	}

	fmt.Printf("User %s subscribed to pro-tier with a quota of 5 requests.\n", userID)

	// 5. Get a limiter for the user.
	limiter, err := manager.GetLimiter(appCtx, userID)
	if err != nil {
		panic(err)
	}

	// 6. Submit tasks for execution.
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		taskNum := i
		go func() {
			defer wg.Done()
			err := limiter.Submit(appCtx, func() {
				fmt.Printf("[%s] [Pro User]  Executing task %d\n", time.Now().Format("15:04:05.000"), taskNum)
				time.Sleep(50 * time.Millisecond) // Simulate work
			})
			if err != nil {
				fmt.Printf("[Pro User] Failed to submit task %d: %v\n", taskNum, err)
			}
		}()
	}

	wg.Wait()

	// 7. Check the user's subscription usage.
	sub, err := manager.GetSubscription(appCtx, userID)
	if err != nil {
		panic(err)
	}

	fmt.Printf("User %s usage: %d/%d\n", userID, sub.Usage, 5)

	// 8. Wait for the subscription to expire.
	fmt.Println("Waiting for subscription to expire...")
	time.Sleep(6 * time.Second)

	// 9. Try to get a limiter for the user again.
	_, err = manager.GetLimiter(appCtx, userID)
	if err != nil {
		fmt.Printf("Successfully blocked user after subscription expired: %v\n", err)
	}
}
```

## Real-World Use Cases

### 🌐 **SaaS API Gateway**
```go
// Free tier: 1000 users share 100 req/min to external APIs
manager.RegisterProfile("free", 
    govalve.WithSharedResource("external-api", rate.Limit(100/60)),
    govalve.WithWorkerPool(50, 1000))

// Pro tier: Each user gets dedicated 500 req/min
manager.RegisterProfile("pro",
    govalve.WithDedicatedResource(rate.Limit(500/60)),
    govalve.WithUsageQuota(10000)) // 10k requests/month
```

### 🗄️ **Database Connection Management**
```go
// Shared pool for basic users
manager.RegisterProfile("basic-db",
    govalve.WithSharedResource("db-pool", rate.Limit(50)),
    govalve.WithWorkerPool(10, 100))

// Dedicated connections for enterprise
manager.RegisterProfile("enterprise-db",
    govalve.WithDedicatedResource(rate.Limit(200)),
    govalve.WithWorkerPool(50, 500))
```

### 🤖 **AI/ML Processing Platform**
```go
// GPU-intensive tasks with strict quotas
manager.RegisterProfile("ai-starter",
    govalve.WithSharedResource("gpu-cluster", rate.Limit(5)),
    govalve.WithUsageQuota(100), // 100 AI requests/month
    govalve.WithWorkerPool(2, 20))
```

### 📊 **Multi-Tenant Analytics**
```go
// Different processing priorities by subscription
manager.RegisterProfile("analytics-basic",
    govalve.WithSharedResource("analytics-engine", rate.Limit(10)),
    govalve.WithWorkerPool(5, 50))

manager.RegisterProfile("analytics-premium",
    govalve.WithDedicatedResource(rate.Limit(100)),
    govalve.WithWorkerPool(20, 200))
```

## Advanced Examples

Detailed examples covering API key pooling, graceful shutdowns, and production deployment patterns can be found in the [`examples/`](examples/) directory.

## Production Deployment

### Database Integration
Implement the `Store` interface for your database:
```go
// Redis implementation
type RedisStore struct { client *redis.Client }

// PostgreSQL implementation  
type PostgreSQLStore struct { db *sql.DB }

// MongoDB implementation
type MongoStore struct { collection *mongo.Collection }
```

### Monitoring & Observability
```go
// Add metrics collection in your Store implementation
func (s *Store) CheckAndIncrementUsage(ctx context.Context, userID string, quota int64) error {
    // Your quota logic here
    metrics.IncrementCounter("govalve.usage", map[string]string{
        "user_id": userID,
        "status": "success",
    })
    return nil
}
```

### Kubernetes Deployment
Govalve works seamlessly in containerized environments with proper graceful shutdown handling.

## Architecture Benefits

- **Horizontal Scaling**: Each service instance manages its own limiters
- **Fault Tolerance**: Isolated failures don't affect other user tiers
- **Cost Optimization**: Efficient resource utilization across user segments
- **Business Model Flexibility**: Easy tier upgrades/downgrades
- **Compliance Ready**: Built-in audit trails through Store interface

## Source Code

The full source code is available on GitHub: [https://github.com/bravian1/govalve](https://github.com/bravian1/govalve)

## Contributing

Contributions are welcome! Please see the [CONTRIBUTING.md](CONTRIBUTING.md) file for details on how to contribute. (Note: This file does not exist yet, but it's good practice to mention it).

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details. 