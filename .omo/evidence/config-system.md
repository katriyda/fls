# Config System

## Files
- `internal/config/config.go` — Config struct with defaults, Load/Save to SQLite, ApplyOverrides, EnvOverrides
- `internal/config/config_test.go` — 10 tests covering defaults, roundtrip, overrides, env, edge cases

## Test Results
```
=== RUN   TestDefaults
--- PASS: TestDefaults (0.00s)
=== RUN   TestSaveLoadRoundtrip
--- PASS: TestSaveLoadRoundtrip (0.01s)
=== RUN   TestApplyOverrides
--- PASS: TestApplyOverrides (0.00s)
=== RUN   TestApplyOverridesZeroValues
--- PASS: TestApplyOverridesZeroValues (0.00s)
=== RUN   TestSaveWithoutDB
--- PASS: TestSaveWithoutDB (0.00s)
=== RUN   TestLoadWithoutDB
--- PASS: TestLoadWithoutDB (0.00s)
=== RUN   TestPartialConfig
--- PASS: TestPartialConfig (0.01s)
=== RUN   TestPersistAcrossMultipleSaves
--- PASS: TestPersistAcrossMultipleSaves (0.01s)
=== RUN   TestEnvOverrides
--- PASS: TestEnvOverrides (0.00s)
=== RUN   TestNewFromDBLoadsDefaultsForMissingKeys
--- PASS: TestNewFromDBLoadsDefaultsForMissingKeys (0.01s)
PASS
ok  	fls/internal/config	2.435s
```

## Configuration Fields
| Key | Default | Type |
|---|---|---|
| port | 8080 | int |
| data_dir | ./data | string |
| token_length | 8 | int |
| max_upload_size | 10737418240 (10GB) | int64 |
| default_expiry | 168h (7d) | time.Duration |
| session_timeout | 24h | time.Duration |
| log_retention_days | 90 | int |
| rate_limit_per_minute | 60 | int |

## Config Layers (lowest to highest priority)
1. Defaults → `Defaults()`
2. SQLite persisted → `Load()` in `New()`
3. Environment variables → `EnvOverrides()` (prefix `FLS_`)
4. CLI flags → `ApplyOverrides()`
