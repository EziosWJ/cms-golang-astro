import assert from "node:assert/strict";
import { test } from "node:test";
import { createUUID } from "../src/lib/uuid.ts";

test("createUUID falls back to getRandomValues when randomUUID is unavailable", () => {
  const source = {
    getRandomValues(bytes) {
      bytes.fill(0);
      return bytes;
    },
  };

  assert.equal(createUUID(source), "00000000-0000-4000-8000-000000000000");
});

test("createUUID prefers the native randomUUID implementation", () => {
  const source = {
    randomUUID: () => "native-uuid",
    getRandomValues() {
      assert.fail("getRandomValues should not be called when randomUUID exists");
    },
  };

  assert.equal(createUUID(source), "native-uuid");
});
