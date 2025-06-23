# Govalve: A Go Rate Limiter & Worker Pool Library

Package `govalve` provides a powerful, flexible mechanism for managing concurrency and rate-limiting for users accessing shared or dedicated resources.

## Overview

In modern scalable systems, a common challenge is managing how multiple concurrent users consume limited resources, such as third-party API keys, database connections, or expensive computational tasks. This library provides a high-level solution to this problem by combining the **Worker Pool** and **Rate Limiter** patterns.

It allows you to easily define different "profiles" for different classes of users (e.g., free-tier, pro-tier). You can configure a group of users to share a single logical resource with a collective rate limit, or you can grant each user their own dedicated resource with a private rate limit.

The library abstracts away the complexity of managing goroutines, channels, and token buckets, providing a simple and safe API to build fair, resilient, and scalable systems.

## Features

*   **Shared Resource Limiting:** Group multiple users (e.g., all "free-tier" users) to share a single resource and a single rate limit.
*   **Dedicated Resource Limiting:** Assign each user (e.g., a "pro-tier" user) their own private worker pool and rate limit.
*   **Concurrency Control:** Limit the number of simultaneously running tasks on a per-user or per-group basis.
*   **Flexible Configuration:** Uses the idiomatic Functional Options pattern for clean, readable, and extensible setup.
*   **Graceful Shutdown:** Safely shut down all worker pools, ensuring all in-flight tasks are completed without leaking goroutines.
*   **Thread-Safe:** Designed from the ground up for safe use in highly concurrent applications.

## Installation

```bash
go get github.com/bravian1/govalve
```

## Quick Start

The primary entry point to the library is the `Manager`. You create a manager, register configuration profiles for your user tiers, and then request `Limiter` instances for your users as they make requests.
```go
package main

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/bravian1/govalve"
	"golang.org/x/time/rate"
)


func main() {
	// Use a context for graceful shutdown of the entire application.
	appCtx, appCancel := context.WithCancel(context.Background())
	defer appCancel()

	// 1. Create a new Manager.
	manager := govalve.NewManager(appCtx)
	defer manager.Shutdown()

	// 2. Register a "free-tier" profile for users sharing a resource.
	// This pool has a combined limit of 2 requests/sec for all users.
	err := manager.RegisterProfile("free-tier",
		govalve.WithSharedResource("shared-free-key", rate.Limit(2)),
		govalve.WithWorkerPool(5, 100), // 5 concurrent workers, 100-task queue
	)
	if err != nil {
		panic(err)
	}

	// 3. Register a "pro-tier" profile for users with a dedicated resource.
	// Each pro user gets their own private limit of 10 requests/sec.
	err = manager.RegisterProfile("pro-tier",
		govalve.WithDedicatedResource(rate.Limit(10)),
		govalve.WithWorkerPool(10, 200), // 10 concurrent workers
	)
	if err != nil {
		panic(err)
	}

	// 4. Get a limiter for a specific user.
	// This would typically happen inside an HTTP handler for each incoming request.
	proUserLimiter, err := manager.GetLimiter("pro-tier", "pro-user-123")
	if err != nil {
		panic(err)
	}

	freeUserLimiter, err := manager.GetLimiter("free-tier", "free-user-abc")
	if err != nil {
		panic(err)
	}

	// 5. Submit tasks for execution.
	var wg sync.WaitGroup
	fmt.Println("Submitting tasks...")

	for i := 0; i < 20; i++ {
		wg.Add(1)
		taskNum := i
		go func() {
			// This task will be throttled by the pro-tier's 10 req/sec limit.
			err := proUserLimiter.Submit(appCtx, func() {
				fmt.Printf("[%s] [Pro User]  Executing task %d\n", time.Now().Format("15:04:05.000"), taskNum)
				time.Sleep(50 * time.Millisecond) // Simulate work
			})
			if err != nil {
				log.Printf("[Pro User] Failed to submit task: %v", err)
			}
			wg.Done()
		}()
	}
    
    for i := 0; i < 5; i++ {
        wg.Add(1)
        taskNum := i
        go func() {
            // This task will be throttled by the free-tier's 2 req/sec limit.
            err := freeUserLimiter.Submit(appCtx, func() {
                fmt.Printf("[%s] [Free User] Executing task %d\n", time.Now().Format("15:04:05.000"), taskNum)
                time.Sleep(50 * time.Millisecond) // Simulate work
            })
            if err != nil {
                log.Printf("[Free User] Failed to submit task: %v", err)
            }
            wg.Done()
        }()
    }

	wg.Wait()
	fmt.Println("All tasks completed.")
}
```
## Examples

More detailed examples covering specific use cases, such as managing a pool of physical API keys or demonstrating graceful shutdowns, can be found in the [`examples/`](examples/) directory of the source repository.

## Source Code

The full source code is available on GitHub: [https://github.com/bravian1/govalve](https://github.com/bravian1/govalve)

## Contributing

Contributions are welcome! Please see the [CONTRIBUTING.md](CONTRIBUTING.md) file for details on how to contribute. (Note: This file does not exist yet, but it's good practice to mention it).

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details. (Note: This file does not exist yet, but it's good practice to mention it).