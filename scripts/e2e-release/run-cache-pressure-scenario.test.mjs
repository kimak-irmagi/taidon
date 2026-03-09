import assert from "node:assert/strict";
import {
  deriveCachePressureMaxBytes,
  validateCachePressureStatus
} from "./run-cache-pressure-scenario.mjs";

function run(name, fn) {
  try {
    fn();
    process.stdout.write(`ok - ${name}\n`);
  } catch (err) {
    process.stderr.write(`not ok - ${name}\n`);
    throw err;
  }
}

run("deriveCachePressureMaxBytes keeps headroom above one-state usage", () => {
  assert.equal(deriveCachePressureMaxBytes(100), 125);
  assert.equal(deriveCachePressureMaxBytes(101), 127);
});

run("deriveCachePressureMaxBytes rejects non-positive usage", () => {
  assert.throws(() => deriveCachePressureMaxBytes(0), /usageBytes must be positive/);
  assert.throws(() => deriveCachePressureMaxBytes(-1), /usageBytes must be positive/);
});

run("validateCachePressureStatus accepts eviction-bearing payload", () => {
  validateCachePressureStatus(
    {
      ok: true,
      cacheSummary: {
        usageBytes: 2048,
        effectiveMaxBytes: 2560
      },
      cacheDetails: {
        reserveBytes: 0,
        lastEviction: {
          evictedCount: 1,
          freedBytes: 1024,
          usageBytesBefore: 4096,
          usageBytesAfter: 2048
        }
      }
    },
    2560
  );
});

run("validateCachePressureStatus rejects payload without lastEviction", () => {
  assert.throws(
    () =>
      validateCachePressureStatus({
        ok: true,
        cacheSummary: {
          usageBytes: 2048,
          effectiveMaxBytes: 2560
        },
        cacheDetails: {
          reserveBytes: 0
        }
      }, 2560),
    /cacheDetails\.lastEviction/
  );
});
