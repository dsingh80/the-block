// Run with: node --test scripts/
//
// Black-box (subprocess) tests, not a refactor of generate_vehicles.mjs into an
// importable module: the script is a one-shot CLI tool, not a library, and every
// run always writes real output as a side effect of just loading the file -- the
// only safe way to test its actual behavior (including the seed CLI parsing) is
// to run it as a real process against a throwaway --out-dir, never the real
// data/vehicles.json.
import { test } from "node:test";
import assert from "node:assert/strict";
import { execFileSync } from "node:child_process";
import { mkdtempSync, readFileSync, rmSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { fileURLToPath } from "node:url";

const scriptPath = fileURLToPath(new URL("./generate_vehicles.mjs", import.meta.url));

function runGenerator(extraArgs) {
  const outDir = mkdtempSync(join(tmpdir(), "vehicles-test-"));
  try {
    execFileSync("node", [scriptPath, `--out-dir=${outDir}`, ...extraArgs], { stdio: "pipe" });
    return JSON.parse(readFileSync(join(outDir, "vehicles.json"), "utf8"));
  } finally {
    rmSync(outDir, { recursive: true, force: true });
  }
}

test("without --seed, two runs produce different vehicle ids", () => {
  const a = runGenerator([]);
  const b = runGenerator([]);
  assert.notDeepEqual(
    a.map((v) => v.id),
    b.map((v) => v.id),
  );
});

test("with an explicit --seed, two runs produce byte-identical output", () => {
  const a = runGenerator(["--seed=12345"]);
  const b = runGenerator(["--seed=12345"]);
  assert.deepEqual(a, b);
});

test("a different explicit --seed produces different output", () => {
  const a = runGenerator(["--seed=1"]);
  const b = runGenerator(["--seed=2"]);
  assert.notDeepEqual(
    a.map((v) => v.id),
    b.map((v) => v.id),
  );
});

test("a non-numeric --seed is rejected", () => {
  assert.throws(() => {
    runGenerator(["--seed=not-a-number"]);
  });
});

test("still produces exactly 200 well-formed records", () => {
  const vehicles = runGenerator(["--seed=42"]);
  assert.equal(vehicles.length, 200);
  for (const v of vehicles) {
    assert.equal(typeof v.id, "string");
    assert.equal(typeof v.vin, "string");
    assert.equal(typeof v.starting_bid, "number");
  }
});
