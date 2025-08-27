package main

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"golang.org/x/time/rate"
	"github.com/bravian1/govalve"
)

// To run a specific example, uncomment the desired main function
// (e.g., mainSharedAndDedicated) and comment out the others.
// The primary `main` function is set to run the most comprehensive demo by default.

func main() {
	// This is the primary, most comprehensive demonstration.
	mainSharedAndDedicated()

	// --- Other Examples (uncomment one to run it) ---
	// mainSimpleShared()
	// mainResourcePooling()
	// mainGracefulShutdown()
	// mainRequestContextTimeout()
}

// EXAMPLE 1: Simple Shared Resource
// Demonstrates the most basic use case: multiple users sharing one resource.
func mainSimpleShared() {
	fmt.Println("--- Running Demo: Simple Shared Resource ---")
	appCtx, cancel := context.WithCancel(context.Background())
	defer cancel()
	manager := govalve.NewManager(appCtx)
	defer manager.Shutdown()

	// All users of this profile share a single 4 req/sec limit.
	manager.RegisterProfile("shared-service",
		govalve.WithSharedResource("the-only-key", rate.Limit(4)),
		govalve.WithWorkerPool(8, 100),
	)

	limiter, _ := manager.GetLimiterForProfile(appCtx, "shared-service", "shared-user")

	var wg sync.WaitGroup
	for i := range 3 { // 3 users
		wg.Add(5) // Each user submits 5 tasks
		userName := fmt.Sprintf("user-%d", i+1)
		for j := 0; j < 5; j++ {
			taskNum := j
			go submitTask(appCtx, limiter, userName, taskNum, &wg)
		}
	}

	wg.Wait()
	fmt.Println("--- Demo Complete: Simple Shared Resource ---")
}

// EXAMPLE 2: Shared and Dedicated Tiers (Primary Demo)
// The most comprehensive example showing different user tiers coexisting.
func mainSharedAndDedicated() {
	fmt.Println("--- Running Demo: Shared vs. Dedicated Tiers ---")
	appCtx, appCancel := context.WithCancel(context.Background())
	var wg sync.WaitGroup
	manager := govalve.NewManager(appCtx)

	// Setup profiles
	manager.RegisterProfile("free-tier", govalve.WithSharedResource("free-key", rate.Limit(2)), govalve.WithWorkerPool(5, 100))
	manager.RegisterProfile("premium-tier", govalve.WithDedicatedResource(rate.Limit(10)), govalve.WithWorkerPool(10, 100))

	freeLimiter, _ := manager.GetLimiterForProfile(appCtx, "free-tier", "free-user")
	premiumLimiter, _ := manager.GetLimiterForProfile(appCtx, "premium-tier", "premium-user-007")

	fmt.Println("Submitting 8 tasks to free tier (2/sec) and 20 tasks to premium (10/sec)...")
	for i := range 20 {
		wg.Add(1)
		go submitTask(appCtx, premiumLimiter, "Premium User", i, &wg)
		if i < 8 {
			wg.Add(1)
			go submitTask(appCtx, freeLimiter, fmt.Sprintf("Free User %d", i%2+1), i, &wg)
		}
	}
	wg.Wait()
	appCancel()
	manager.Shutdown()
	fmt.Println("--- Demo Complete: Shared vs. Dedicated Tiers ---")
}

// EXAMPLE 3: Managing a Pool of Physical Keys
// Shows how to use the library to rate-limit access to your OWN pool
// of multiple physical resources (e.g., 3 different API keys).
func mainResourcePooling() {
	fmt.Println("--- Running Demo: Managing a Pool of API Keys ---")
	appCtx, cancel := context.WithCancel(context.Background())
	defer cancel()
	manager := govalve.NewManager(appCtx)
	defer manager.Shutdown()

	// This limiter's job is just to control the *overall* request rate to 5/sec.
	manager.RegisterProfile("key-pool-limiter",
		govalve.WithSharedResource("my-key-pool", rate.Limit(5)),
		govalve.WithWorkerPool(10, 100),
	)
	limiter, _ := manager.GetLimiterForProfile(appCtx, "key-pool-limiter", "pool-user")

	// Your application's key pool, managed outside the library.
	apiKeyPool := make(chan string, 3)
	apiKeyPool <- "key-A-xxxxx"
	apiKeyPool <- "key-B-yyyyy"
	apiKeyPool <- "key-C-zzzzz"

	var wg sync.WaitGroup
	for i := range 15 {
		wg.Add(1)
		taskNum := i
		// The task submitted to the limiter contains the key pooling logic.
		err := limiter.Submit(appCtx, func() {
			apiKey := <-apiKeyPool // Borrow a key
			fmt.Printf("[%s] -> Task %d using [KEY: %s]\n", time.Now().Format("15:04:05.000"), taskNum, apiKey)
			time.Sleep(200 * time.Millisecond) // Simulate API call
			apiKeyPool <- apiKey               // Return key to pool
			wg.Done()
		})
		if err != nil {
			log.Printf("Failed to submit task: %v", err)
			wg.Done()
		}
	}
	wg.Wait()
	fmt.Println("--- Demo Complete: Managing a Pool of API Keys ---")
}

// EXAMPLE 4: Graceful Shutdown
// Demonstrates how the system handles being shut down while tasks are running.
func mainGracefulShutdown() {
	fmt.Println("--- Running Demo: Graceful Shutdown ---")
	appCtx, cancel := context.WithCancel(context.Background())
	manager := govalve.NewManager(appCtx)

	manager.RegisterProfile("long-tasks", govalve.WithSharedResource("long-running", rate.Limit(5)), govalve.WithWorkerPool(5, 5))
	limiter, _ := manager.GetLimiterForProfile(appCtx, "long-tasks", "shutdown-user")

	var wg sync.WaitGroup
	fmt.Println("Submitting 10 long-running tasks...")
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go submitTask(appCtx, limiter, "Shutdown-Test", i, &wg)
	}

	time.Sleep(1 * time.Second) // Let some tasks start
	fmt.Println("\n>>> INITIATING SHUTDOWN! Canceling context... <<<")
	cancel() // Trigger the shutdown

	// Try to submit one more task after shutdown has started
	wg.Add(1)
	go submitTask(appCtx, limiter, "Late-Submission", 99, &wg)

	wg.Wait()
	manager.Shutdown()
	fmt.Println("--- Demo Complete: Graceful Shutdown ---")
}

// EXAMPLE 5: Per-Request Timeout
// Demonstrates how a user can provide a timeout for a single Submit call.
func mainRequestContextTimeout() {
	fmt.Println("--- Running Demo: Per-Request Timeout ---")
	appCtx, cancel := context.WithCancel(context.Background())
	defer cancel()
	manager := govalve.NewManager(appCtx)
	defer manager.Shutdown()

	// A very busy limiter with a full queue to force submission blocking
	manager.RegisterProfile("busy-limiter", govalve.WithSharedResource("busy", rate.Limit(2)), govalve.WithWorkerPool(1, 1))
	limiter, _ := manager.GetLimiterForProfile(appCtx, "busy-limiter", "timeout-user")
	var wg sync.WaitGroup

	// Fill the worker and the queue
	for i := 0; i < 2; i++ {
		wg.Add(1)
		limiter.Submit(appCtx, func() {
			fmt.Println("Initial task running, blocking queue...")
			time.Sleep(3 * time.Second)
			wg.Done()
		})
	}

	time.Sleep(100 * time.Millisecond) // Ensure initial tasks are queued

	// This submission should time out because the queue is full and won't clear in time.
	fmt.Println("Submitting a task with a short 1-second timeout...")
	reqCtx, reqCancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer reqCancel()

	err := limiter.Submit(reqCtx, func() {
		// This part will never run
		fmt.Println("This should not be printed!")
	})

	if err != nil {
		fmt.Printf("Successfully caught expected error: %v\n", err)
	} else {
		fmt.Println("Error: Task was submitted but should have timed out!")
	}

	wg.Wait()
	fmt.Println("--- Demo Complete: Per-Request Timeout ---")
}

// Helper function to reduce boilerplate in demos
func submitTask(ctx context.Context, l govalve.Limiter, user string, taskNum int, wg *sync.WaitGroup) {
	err := l.Submit(ctx, func() {
		fmt.Printf("[%s] -> Task %2d for [ %s ] starting...\n", time.Now().Format("15:04:05.000"), taskNum, user)
		time.Sleep(400 * time.Millisecond) 
		wg.Done()
	})

	if err != nil {
		fmt.Printf("[SYSTEM] Task %d for [ %s ] failed to submit: %v\n", taskNum, user, err)
		wg.Done()
	}
}
