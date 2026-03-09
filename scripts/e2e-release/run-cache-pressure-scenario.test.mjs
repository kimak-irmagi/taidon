import assert from "node:assert/strict";
import {
  buildConfigSetCommand,
  buildListInstancesCommand,
  buildPrepareCommand,
  buildRmCommand,
  deriveCachePressureMaxBytes,
  parseInstanceIDs,
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

run("buildPrepareCommand keeps cache-pressure scenario prepare-only", () => {
  assert.deepEqual(
    buildPrepareCommand({
      sqlrsPath: "/tmp/sqlrs",
      workspaceDir: "/tmp/workspace",
      timeout: "30m",
      scenario: {
        id: "cache-pressure-chinook",
        image: "postgres:17",
        prepareArgs: ["-f", "prepare.sql"]
      }
    }),
    ["/tmp/sqlrs", "--timeout", "30m", "--workspace", "/tmp/workspace", "prepare:psql", "--image", "postgres:17", "--", "-f", "prepare.sql"]
  );
});

run("buildListInstancesCommand and buildRmCommand target local workspace CLI", () => {
  assert.deepEqual(
    buildListInstancesCommand({ sqlrsPath: "/tmp/sqlrs", workspaceDir: "/tmp/workspace" }),
    ["/tmp/sqlrs", "--output", "json", "--workspace", "/tmp/workspace", "ls", "--instances"]
  );
  assert.deepEqual(
    buildRmCommand({ sqlrsPath: "/tmp/sqlrs", workspaceDir: "/tmp/workspace", idPrefix: "inst-1" }),
    ["/tmp/sqlrs", "--workspace", "/tmp/workspace", "rm", "inst-1"]
  );
});

run("parseInstanceIDs extracts all listed instance ids", () => {
  assert.deepEqual(parseInstanceIDs({ instances: [{ instance_id: "inst-1" }, { instance_id: "inst-2" }] }), ["inst-1", "inst-2"]);
  assert.throws(() => parseInstanceIDs({ instances: [] }), /at least one instance/);
  assert.throws(() => parseInstanceIDs({ instances: [{ instance_id: "" }] }), /instance_id/);
});

run("buildConfigSetCommand encodes string values as JSON literals", () => {
  assert.deepEqual(
    buildConfigSetCommand({
      sqlrsPath: "/tmp/sqlrs",
      workspaceDir: "/tmp/workspace",
      pathName: "cache.capacity.minStateAge",
      value: "0s"
    }),
    ["/tmp/sqlrs", "--workspace", "/tmp/workspace", "config", "set", "cache.capacity.minStateAge", "\"0s\""]
  );
});

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
