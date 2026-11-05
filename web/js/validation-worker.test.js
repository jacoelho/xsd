import assert from "node:assert/strict";
import test from "node:test";

import {
  ValidationCanceledError,
  ValidationSupersededError,
  ValidationTimeoutError,
  ValidationWorkerClient,
} from "./validation-worker.js";

const limits = Object.freeze({
  maxXMLBytes: 2_097_152,
  maxFormattedXMLBytes: 2_097_152,
  maxXSDBytes: 1_048_576,
  maxValidationErrors: 100,
});

test("starts from the worker-owned immutable limit catalog", async () => {
  const factory = fakeWorkerFactory();
  const client = new ValidationWorkerClient("worker.js", { workerFactory: factory.create });
  const ready = client.start();

  factory.workers[0].receive({ type: "ready", limits });

  assert.deepEqual(await ready, limits);
  assert.equal(Object.isFrozen(client.state), true);
  assert.equal(Object.isFrozen(client.state.limits), true);
  assert.equal(client.state.phase, "ready");
  client.dispose();
});

test("rejects oversized UTF-8 input before posting it to a ready worker", async () => {
  const factory = fakeWorkerFactory();
  const client = new ValidationWorkerClient("worker.js", { workerFactory: factory.create });
  const starting = client.start();
  const worker = factory.workers[0];
  worker.receive({ type: "ready", limits: { ...limits, maxXMLBytes: 2 } });
  await starting;

  await assert.rejects(client.validate("€", "schema"), {
    name: "RangeError",
    message: "XML exceeds 2 bytes limit",
  });
  assert.deepEqual(worker.messages, []);
  client.dispose();
});

test("rejects oversized queued input after limits arrive without cloning it", async () => {
  const factory = fakeWorkerFactory();
  const client = new ValidationWorkerClient("worker.js", { workerFactory: factory.create });
  const starting = client.start();
  const queued = client.validate("€", "schema");
  const worker = factory.workers[0];
  worker.receive({ type: "ready", limits: { ...limits, maxXMLBytes: 2 } });

  await starting;
  await assert.rejects(queued, RangeError);
  assert.deepEqual(worker.messages, []);
  assert.equal(client.state.phase, "ready");
  client.dispose();
});

test("runs one request and retains only the latest pending request", async () => {
  const factory = fakeWorkerFactory();
  const client = new ValidationWorkerClient("worker.js", { workerFactory: factory.create });
  const ready = client.start();
  const worker = factory.workers[0];
  worker.receive({ type: "ready", limits });
  await ready;

  const first = client.validate("one", "schema");
  const superseded = client.validate("two", "schema");
  const latest = client.validate("three", "schema");
  await assert.rejects(superseded, ValidationSupersededError);

  assert.deepEqual(worker.messages.map((message) => message.xml), ["one"]);
  const firstRequest = worker.messages[0];
  worker.receive({
    type: "result",
    requestId: firstRequest.requestId,
    flow: { result: { status: "valid" }, xml: "one" },
  });

  assert.deepEqual(await first, { result: { status: "valid" }, xml: "one" });
  assert.deepEqual(worker.messages.map((message) => message.xml), ["one", "three"]);
  const latestRequest = worker.messages[1];
  worker.receive({
    type: "result",
    requestId: latestRequest.requestId,
    flow: { result: { status: "invalid", errors: [{ message: "bad" }] }, xml: "three" },
  });
  assert.equal((await latest).result.status, "invalid");
  assert.equal(client.state.phase, "ready");
  client.dispose();
});

test("cancel terminates active work, rejects queued work, and starts a fresh worker", async () => {
  const factory = fakeWorkerFactory();
  const client = new ValidationWorkerClient("worker.js", { workerFactory: factory.create });
  const ready = client.start();
  const firstWorker = factory.workers[0];
  firstWorker.receive({ type: "ready", limits });
  await ready;

  const active = client.validate("one", "schema");
  const pending = client.validate("two", "schema");
  const restarted = client.cancel("input cleared");

  await assert.rejects(active, ValidationCanceledError);
  await assert.rejects(pending, ValidationCanceledError);
  assert.equal(firstWorker.terminated, true);
  assert.equal(factory.workers.length, 2);
  factory.workers[1].receive({ type: "ready", limits });
  await restarted;
  assert.equal(client.state.phase, "ready");
  client.dispose();
});

test("cancel rejects an in-flight startup without leaving an orphaned promise", async () => {
  const factory = fakeWorkerFactory();
  const client = new ValidationWorkerClient("worker.js", { workerFactory: factory.create });
  const starting = client.start();
  const restarted = client.cancel("restart");

  await assert.rejects(starting, ValidationCanceledError);
  assert.equal(factory.workers[0].terminated, true);
  factory.workers[1].receive({ type: "ready", limits });
  await restarted;
  client.dispose();
});

test("startup timeout rejects queued work and permits a fresh worker", async () => {
  const factory = fakeWorkerFactory();
  const client = new ValidationWorkerClient("worker.js", {
    workerFactory: factory.create,
    timeoutMs: 5,
  });
  const starting = client.start();
  const queued = client.validate("one", "schema");

  await Promise.all([
    assert.rejects(starting, ValidationTimeoutError),
    assert.rejects(queued, ValidationTimeoutError),
  ]);
  assert.equal(factory.workers[0].terminated, true);
  assert.equal(client.state.phase, "failed");

  const restarted = client.start();
  factory.workers[1].receive({ type: "ready", limits });
  await restarted;
  assert.equal(client.state.phase, "ready");
  client.dispose();
});

test("timeout terminates a stuck worker and preserves the latest pending request", async () => {
  const factory = fakeWorkerFactory();
  const client = new ValidationWorkerClient("worker.js", {
    workerFactory: factory.create,
    timeoutMs: 5,
  });
  const ready = client.start();
  const firstWorker = factory.workers[0];
  firstWorker.receive({ type: "ready", limits });
  await ready;

  const timedOut = client.validate("one", "schema");
  const latest = client.validate("two", "schema");
  await assert.rejects(timedOut, ValidationTimeoutError);
  assert.equal(firstWorker.terminated, true);

  const replacement = factory.workers[1];
  replacement.receive({ type: "ready", limits });
  assert.equal(replacement.messages[0].xml, "two");
  replacement.receive({
    type: "result",
    requestId: replacement.messages[0].requestId,
    flow: { result: { status: "valid" }, xml: "two" },
  });
  assert.equal((await latest).result.status, "valid");
  client.dispose();
});

test("rejects a result that does not belong to the active request", async () => {
  const factory = fakeWorkerFactory();
  const client = new ValidationWorkerClient("worker.js", { workerFactory: factory.create });
  const ready = client.start();
  const worker = factory.workers[0];
  worker.receive({ type: "ready", limits });
  await ready;

  const active = client.validate("one", "schema");
  worker.receive({
    type: "result",
    requestId: worker.messages[0].requestId + 1,
    flow: { result: { status: "valid" }, xml: "one" },
  });

  await assert.rejects(active, /Invalid validation worker request ID/);
  assert.equal(worker.terminated, true);
  assert.equal(client.state.phase, "failed");
  client.dispose();
});

test("rejects readiness when queued dispatch fails", async () => {
  const factory = fakeWorkerFactory({ postError: new Error("post failed") });
  const client = new ValidationWorkerClient("worker.js", { workerFactory: factory.create });
  const starting = client.start();
  const queued = client.validate("one", "schema");
  factory.workers[0].receive({ type: "ready", limits });

  await assert.rejects(starting, /post failed/);
  await assert.rejects(queued, /post failed/);
  assert.equal(client.state.phase, "failed");
  client.dispose();
});

test("observer failures cannot unwind worker lifecycle methods", async (t) => {
  const observerErrors = [];
  const previousReportError = globalThis.reportError;
  globalThis.reportError = (err) => observerErrors.push(err);
  t.after(() => {
    if (previousReportError === undefined) {
      delete globalThis.reportError;
    } else {
      globalThis.reportError = previousReportError;
    }
  });
  const factory = fakeWorkerFactory();
  const client = new ValidationWorkerClient("worker.js", {
    workerFactory: factory.create,
    onStateChange() {
      throw new Error("observer failed");
    },
  });

  const starting = client.start();
  factory.workers[0].receive({ type: "ready", limits });
  await starting;
  const validation = client.validate("one", "schema");
  factory.workers[0].receive({
    type: "result",
    requestId: factory.workers[0].messages[0].requestId,
    flow: { result: { status: "valid" }, xml: "one" },
  });
  await validation;
  const restarted = client.cancel("restart");
  factory.workers[1].receive({ type: "ready", limits });
  await restarted;
  assert.doesNotThrow(() => client.dispose());
  await Promise.resolve();
  assert.ok(observerErrors.length >= 6);
  assert.ok(observerErrors.every((err) => err instanceof Error && err.message === "observer failed"));
});

class FakeWorker {
  messages = [];
  onerror = null;
  onmessage = null;
  terminated = false;

  constructor(postError = null) {
    this.postError = postError;
  }

  postMessage(message) {
    if (this.postError !== null) throw this.postError;
    this.messages.push(message);
  }

  receive(data) {
    this.onmessage?.({ data });
  }

  terminate() {
    this.terminated = true;
  }
}

function fakeWorkerFactory({ postError = null } = {}) {
  const workers = [];
  return {
    workers,
    create() {
      const worker = new FakeWorker(postError);
      workers.push(worker);
      return worker;
    },
  };
}
