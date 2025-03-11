# Changelog

All notable changes to the Lemmings project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.1.1] - 2025-03-11

### Added

- Enhanced middleware support
- Improved resource monitoring
- New examples for custom worker implementations

### Fixed

- Prevent task queue deadlocks during high
concurrency
- Middleware chain execution order consistency
- Resource control memory tracking accuracy

## [0.1.0] - 2025-03-11

### Added

- Dynamic worker scaling based on resource usage
- Priority queue for task scheduling
- Middleware system for cross-cutting concerns
- Task retry capabilities with backoff
- Comprehensive monitoring and statistics
- Basic metrics collection
- Simple worker pooling
- Task execution with timeouts
- Initial release with core functionality
- Basic worker pool implementation
- Simple task interface
- Manager for coordinating workers

### Changed

- Refactored worker pool for better performance
- Improved error handling and propagation
- Enhanced context support for proper

### Fixed

- Memory leaks in long-running worker processes
- Race conditions in task result handling
- Goroutine leaks during shutdown
- Worker initialization issues
- Task validation logic
