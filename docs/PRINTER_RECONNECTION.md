# Printer connection recovery

## Configuration

```sh
PRINTER_RECONNECT_INTERVAL=10s
PRINTER_RECONNECT_MAX_CONCURRENT=4
```

Both are backend startup settings. Interval must be a Go duration of at least `2s`; concurrency must be an integer from `1` to `256`. The upper bound prevents accidental unbounded worker allocation; it is not a recommendation to run 256 concurrent attempts. Invalid values must stop startup with a configuration error. Restart after changing them.

The connection-attempt deadline is interval minus one second: defaults are 10 seconds / 9 seconds. Individual transport operations may fail earlier (HTTP clients also have their own per-request timeout). The interval is a minimum between starts for the same printer, not an additional sleep after the timeout. Waiting for capacity may increase the actual interval. A timeout must cancel actual network work, not merely abandon a goroutine while it continues dialing.

Concurrency is a bound on simultaneous connection attempts, not on the number of printers registered or already connected. Four is a conservative default, not a benchmarked maximum. More offline printers increase waiting time rather than creating unlimited concurrent attempts. For example, 100 unavailable printers with 10 workers and full 9-second attempts take roughly 90 seconds per round, plus scheduling/cleanup overhead; the 10-second setting is not a fleet-wide recovery guarantee. Established telemetry connections have their own resource costs outside this attempt limit. The MQTT library's immediate disconnect initiates asynchronous cleanup; the worker limit is not a strict cap on all transient cleanup goroutines or sockets.

Manual requests join an existing attempt for the same printer rather than opening another one. Cancelling an HTTP caller stops its wait, not the shared recovery attempt; the attempt retains the configured deadline and is cancelled on scheduler shutdown.

## Expected recovery

A printer powered off remains eligible for automatic recovery when it is powered back on. The operator should not need to click Reconnect or restart PicoFarm. The device must first finish booting and make its network service available; scheduling and negotiation add delay after that point.

An operational `error` is not by itself evidence of broken communication. Connected printers reporting errors must remain connected. Manual, maintenance-mode and unsupported connection types are excluded from automatic attempts.

Recovery establishes communication and obtains status; it must not replay start, resume or other physical-action commands. Losing communication is not proof of print completion. Reconnection is not a durable reconciliation mechanism for prints interrupted by power loss.

Existing workflow limitations are separate: the unified queue can mark communication loss as failure, while `PrintJob` records can remain active. Queue reconciliation uses a >=99% progress heuristic for completion; that is not proof of a physically finished part after power loss. This connection-recovery change does not redesign those policies. Inspect interrupted work before manually retrying or approving finished stock.

## Validation and operational limits

Use temporary database records and local fake HTTP/MQTT/WebSocket services. Exercise power-off/unavailable then available, attempt deadlines, capacity saturation, manual/automatic overlap, stale callbacks from retired clients, maintenance and shutdown. Keep existing manager nonblocking-state-read tests and run race detection.

Established HTTP polling runs every five seconds; loss detection can take a polling interval plus the request duration. MQTT loss detection also depends on transport/keepalive. Recovery latency includes that detection time, not only the reconnect interval.

Generation checks reject retired clients' later callbacks, but do not undo a callback that already passed validation and began executing when disconnect/shutdown occurs. Downstream automation is not transactional with connection retirement.

CPU/RAM and recovery-time behavior on real devices are not established by functional tests. Before raising concurrency, measure process CPU/RSS, sockets, goroutines, API latency under outage and recovery time. A very short timeout may prevent slow devices or TLS services from connecting. No fixed concurrency value guarantees zero performance impact.
