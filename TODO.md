# Govalve Subscription Features TODO

This file outlines the tasks required to implement subscription management features in the `govalve` library.

## Phase 1: Core Data Structures and Interfaces

- [x] Create a new `subscription.go` file.
- [x] Define the `Subscription` struct in `subscription.go` with fields for `UserID`, `ProfileName`, `Status` (e.g., Active, Expired, Canceled), `StartDate`, `EndDate`, and `Usage`.
- [x] Define the `Store` interface in a new `store.go` file with methods like `GetSubscription(ctx, userID)` and `SaveSubscription(ctx, subscription)`.

## Phase 2: Manager and Subscription Logic

- [x] Modify the `Manager` struct in `manager.go` to include a `Store` instance.
- [x] Create a `WithStore(store Store)` option in `manager.go` to allow users to inject their store implementation.
- [x] Implement a new `Subscribe(ctx, userID, profileName, duration)` method on the `Manager`.
- [x] Implement a `GetSubscription(ctx, userID)` method on the `Manager` that retrieves the subscription from the store.
- [x] Modify the `GetLimiter` method in `manager.go` to first get the user's subscription and check if it's active before returning a limiter.

## Phase 3: Limiter Integration

- [x] Modify the `limiterConfig` in `config.go` to include subscription-related parameters like usage quotas.
- [x] Update the `Limiter.Submit()` method in `limiter.go` to check the subscription's status and usage quota before processing a task.
- [x] Add logic to the `Limiter` to report back to the `Manager` when usage occurs, so the `Manager` can update the subscription in the store.

## Phase 4: Documentation and Examples

- [x] Update `README.md` to explain the new subscription management features and how to use them.
- [x] Create a new example file in the `examples/` directory (`example2.go` or similar) that demonstrates how to use the subscription features with a mock `Store` implementation.
- [x] Add comments to the new code to explain the design and implementation choices.
