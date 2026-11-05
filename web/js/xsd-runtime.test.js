import assert from "node:assert/strict";
import test from "node:test";

import { startRuntime } from "./xsd-runtime.js";

const limits = { maxXMLBytes: 1, maxFormattedXMLBytes: 1, maxXSDBytes: 1, maxValidationErrors: 1 };

test("posts ready, then fatal when the Go program exits", async () => {
  const runtime = fakeRuntime();
  const api = await startRuntime(runtime.options);
  assert.deepEqual(runtime.posted, [{ type: "ready", limits }]);
  assert.equal(typeof api.validateXML, "function");

  runtime.exit.resolve();
  await settled();
  assert.deepEqual(runtime.posted, [
    { type: "ready", limits },
    { type: "fatal", error: "Go runtime exited" },
  ]);
});

test("posts the runtime error when the Go program fails", async () => {
  const runtime = fakeRuntime();
  await startRuntime(runtime.options);

  runtime.exit.reject(new Error("wasm trap"));
  await settled();
  assert.deepEqual(runtime.posted.at(-1), { type: "fatal", error: "Error: wasm trap" });
  assert.equal(runtime.posted.filter((message) => message.type === "fatal").length, 1);
});

test("posts one fatal when the API is missing and the program then exits", async () => {
  const runtime = fakeRuntime({ globals: { xsdLimits: JSON.stringify(limits) } });
  await assert.rejects(startRuntime(runtime.options), /WASM API did not initialize/);

  runtime.exit.resolve();
  await settled();
  assert.deepEqual(runtime.posted, [{ type: "fatal", error: "Error: WASM API did not initialize" }]);
});

test("posts one fatal when instantiation fails without running Go", async () => {
  const runtime = fakeRuntime({ instantiateError: new Error("fetch failed") });
  await assert.rejects(startRuntime(runtime.options), /fetch failed/);

  await settled();
  assert.equal(runtime.runs, 0);
  assert.deepEqual(runtime.posted, [{ type: "fatal", error: "Error: fetch failed" }]);
});

function fakeRuntime({
  globals = {
    xsdLimits: JSON.stringify(limits),
    validateXML() {},
    formatXML() {},
  },
  instantiateError = null,
} = {}) {
  const posted = [];
  const exit = {};
  const exited = new Promise((resolve, reject) => {
    exit.resolve = resolve;
    exit.reject = reject;
  });
  const runtime = {
    posted,
    exit,
    runs: 0,
    options: {
      createGo: () => ({
        importObject: {},
        run() {
          runtime.runs++;
          return exited;
        },
      }),
      async instantiate() {
        if (instantiateError !== null) throw instantiateError;
        return {};
      },
      globals,
      post: (message) => posted.push(message),
    },
  };
  return runtime;
}

function settled() {
  return new Promise((resolve) => setTimeout(resolve, 0));
}
