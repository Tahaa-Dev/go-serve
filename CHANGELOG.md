# Changelog

## v0.3.2 - 2026-06-17

### Fixed

- Major deadlock bug in cache
- Optimized addition of new entries into cache
- Optimized connection dropping

---

## v0.3.1 - 2026-06-17

### Fixed

- Logging to make it more efficient as it used too much CPU power

---

## v0.3.0 - 2026-06-16

### Added

- `DELETE /` route for deleting files from disk and cache with auth and logging
- `-m` flag for setting system rlimit on Unix systems
- More perfromance improvements
- Better RPS (requests per second) and concurrent requests scalability using less lock contentions and leveraging atomics
- Smaller memory footprint
- Much better architecture

---

## v0.2.0 - 2026-06-11

### Added

- `POST /` route for creating new files and adding them to cache with and logging
- `PUT /` route for updating existing files and adding them to cache/updating their existing cache entries with auth and logging
- Test suite covering most of the core functionality
- GitHub Actions CI/CD
- Architectural and perfromance improvements
