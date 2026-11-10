import { isValidationFlow } from "./validation-flow.js";

export class ValidationCanceledError extends Error {
  constructor(message = "Validation canceled") {
    super(message);
    this.name = "ValidationCanceledError";
  }
}

export class ValidationSupersededError extends Error {
  constructor() {
    super("Validation superseded by a newer request");
    this.name = "ValidationSupersededError";
  }
}

export class ValidationTimeoutError extends Error {
  constructor(timeoutMs) {
    super(`Validation exceeded ${timeoutMs} ms`);
    this.name = "ValidationTimeoutError";
  }
}

// ValidationWorkerClient owns one worker's lifecycle. #state is the only
// lifecycle record; each variant carries exactly the resources it owns:
//
//   idle      {}                          no worker yet
//   loading   {worker, timer, ready}      awaiting the worker's ready message
//   ready     {worker}                    idle worker, no active request
//   running   {worker, timer, active}     one request posted to the worker
//   failed    {error}                     worker released; start or validate respawns
//   disposed  {}                          terminal
//
// Legal progress: idle|failed -> loading (start, validate);
// loading -> ready -> running -> ready; running -> loading (request timeout);
// any live phase -> loading (cancel); loading|ready|running -> failed;
// any phase -> disposed. #leave is the only release of a state's worker,
// timer, active request, and readiness promise. The latest pending request is
// orthogonal to the phase and survives respawns until it runs or is rejected.
export class ValidationWorkerClient {
  #generation = 0;
  #limits = null;
  #onStateChange;
  #pending = null;
  #sequence = 0;
  #state = IDLE;
  #timeoutMs;
  #workerFactory;
  #workerURL;

  constructor(workerURL, {
    timeoutMs = 30_000,
    workerFactory = defaultWorkerFactory,
    onStateChange = () => {},
  } = {}) {
    if (!workerURL) throw new TypeError("workerURL is required");
    if (!Number.isSafeInteger(timeoutMs) || timeoutMs <= 0) {
      throw new RangeError("timeoutMs must be a positive safe integer");
    }
    if (typeof workerFactory !== "function") throw new TypeError("workerFactory must be a function");
    if (typeof onStateChange !== "function") throw new TypeError("onStateChange must be a function");

    this.#workerURL = workerURL;
    this.#timeoutMs = timeoutMs;
    this.#workerFactory = workerFactory;
    this.#onStateChange = onStateChange;
  }

  // limits is the catalog announced by the most recent ready worker. The
  // catalog is fixed by the WASM build behind the one worker URL, so it stays
  // valid while a replacement worker loads or after a failure; each ready
  // message replaces it. It is null only before any worker became ready.
  get state() {
    return Object.freeze({
      phase: this.#state.phase,
      busy: this.#state.phase === "running" || this.#pending !== null,
      limits: this.#limits,
    });
  }

  start() {
    switch (this.#state.phase) {
      case "disposed":
        return Promise.reject(disposedError());
      case "ready":
      case "running":
        return Promise.resolve(this.#limits);
      case "loading":
        return this.#state.ready.promise;
      default:
        return this.#spawn();
    }
  }

  validate(xml, xsd) {
    if (typeof xml !== "string" || typeof xsd !== "string") {
      return Promise.reject(new TypeError("xml and xsd must be strings"));
    }
    if (this.#state.phase === "disposed") return Promise.reject(disposedError());

    const limitError = requestLimitError(xml, xsd, this.#limits);
    if (limitError !== null) return Promise.reject(limitError);

    const request = deferred();
    request.id = ++this.#sequence;
    request.xml = xml;
    request.xsd = xsd;
    this.#pending?.reject(new ValidationSupersededError());
    this.#pending = request;

    if (this.#state.phase === "idle" || this.#state.phase === "failed") {
      this.#spawn();
    } else {
      this.#drain();
      this.#emitState();
    }
    return request.promise;
  }

  cancel(message) {
    if (this.#state.phase === "disposed") return Promise.resolve(null);
    const err = new ValidationCanceledError(message);
    this.#rejectPending(err);
    this.#leave(err);
    return this.#spawn();
  }

  dispose() {
    if (this.#state.phase === "disposed") return;
    const err = disposedError();
    this.#rejectPending(err);
    this.#leave(err);
    this.#state = DISPOSED;
    this.#emitState();
  }

  // #spawn enters loading from a state that owns no worker, and returns the
  // new worker's readiness promise.
  #spawn() {
    const generation = ++this.#generation;
    const ready = deferred();
    ready.promise.catch(() => {});
    const loading = { phase: "loading", worker: null, timer: null, ready };
    this.#state = loading;
    try {
      const worker = this.#workerFactory(this.#workerURL);
      loading.worker = worker;
      worker.onmessage = (event) => this.#receive(generation, event.data);
      worker.onerror = (event) => {
        this.#fail(generation, new Error(event?.message || "Validation worker failed"));
      };
      loading.timer = setTimeout(() => this.#initializationTimedOut(generation), this.#timeoutMs);
    } catch (err) {
      this.#fail(generation, asError(err));
      return ready.promise;
    }
    this.#emitState();
    return ready.promise;
  }

  #receive(generation, message) {
    if (generation !== this.#generation) return;
    if (!message || typeof message !== "object" || Array.isArray(message)) {
      this.#fail(generation, new Error("Invalid validation worker message"));
      return;
    }

    switch (message.type) {
      case "ready":
        this.#workerReady(generation, message.limits);
        return;
      case "result":
        this.#completeRequest(generation, message.requestId, message.flow);
        return;
      case "failure":
        this.#rejectRequest(
          generation,
          message.requestId,
          new Error(nonEmptyString(message.error) || "Validation failed"),
        );
        return;
      case "fatal":
        this.#fail(generation, new Error(nonEmptyString(message.error) || "Validation worker failed"));
        return;
      default:
        this.#fail(generation, new Error("Invalid validation worker message"));
    }
  }

  #workerReady(generation, rawLimits) {
    const loading = this.#state;
    if (loading.phase !== "loading") {
      this.#fail(generation, new Error("Unexpected validation worker readiness"));
      return;
    }
    let limits;
    try {
      limits = normalizeLimits(rawLimits);
    } catch (err) {
      this.#fail(generation, asError(err));
      return;
    }
    clearTimeout(loading.timer);
    this.#limits = limits;
    this.#state = { phase: "ready", worker: loading.worker };
    this.#drain();
    // Readiness settles only after queued work was dispatched, so a dispatch
    // failure rejects the start that was waiting for this worker.
    if (this.#state.phase === "failed") {
      loading.ready.reject(this.#state.error);
      return;
    }
    loading.ready.resolve(limits);
    this.#emitState();
  }

  #drain() {
    const ready = this.#state;
    if (ready.phase !== "ready" || this.#pending === null) return;
    const request = this.#pending;
    this.#pending = null;
    const limitError = requestLimitError(request.xml, request.xsd, this.#limits);
    if (limitError !== null) {
      request.reject(limitError);
      return;
    }
    this.#state = {
      phase: "running",
      worker: ready.worker,
      active: request,
      timer: setTimeout(() => this.#requestTimedOut(request.id), this.#timeoutMs),
    };
    try {
      ready.worker.postMessage({
        type: "validate",
        requestId: request.id,
        xml: request.xml,
        xsd: request.xsd,
      });
    } catch (err) {
      this.#fail(this.#generation, asError(err));
    }
  }

  #completeRequest(generation, requestId, flow) {
    if (!this.#matchesActive(requestId)) {
      this.#fail(generation, new Error("Invalid validation worker request ID"));
      return;
    }
    if (!isValidationFlow(flow)) {
      this.#fail(generation, new Error("Invalid validation worker result"));
      return;
    }
    this.#finishActive().resolve(flow);
    this.#drain();
    this.#emitState();
  }

  #rejectRequest(generation, requestId, err) {
    if (!this.#matchesActive(requestId)) {
      this.#fail(generation, new Error("Invalid validation worker request ID"));
      return;
    }
    this.#finishActive().reject(err);
    this.#drain();
    this.#emitState();
  }

  // A stuck worker is replaced; the latest pending request runs on the new one.
  #requestTimedOut(requestId) {
    if (!this.#matchesActive(requestId)) return;
    this.#leave(new ValidationTimeoutError(this.#timeoutMs));
    this.#spawn();
  }

  #initializationTimedOut(generation) {
    if (generation !== this.#generation || this.#state.phase !== "loading") return;
    this.#fail(generation, new ValidationTimeoutError(this.#timeoutMs));
  }

  #matchesActive(requestId) {
    return this.#state.phase === "running" && Number.isSafeInteger(requestId) &&
      this.#state.active.id === requestId;
  }

  // #finishActive moves running -> ready and returns the request to settle.
  #finishActive() {
    const running = this.#state;
    clearTimeout(running.timer);
    this.#state = { phase: "ready", worker: running.worker };
    return running.active;
  }

  #fail(generation, err) {
    if (generation !== this.#generation) return;
    this.#rejectPending(err);
    this.#leave(err);
    this.#state = { phase: "failed", error: err };
    this.#emitState();
  }

  // #leave releases everything the current state owns and invalidates its
  // worker's callbacks. The caller installs the next state.
  #leave(err) {
    const current = this.#state;
    this.#generation++;
    if (current.timer !== undefined) clearTimeout(current.timer);
    if (current.worker) {
      current.worker.onmessage = null;
      current.worker.onerror = null;
      current.worker.terminate();
    }
    if (current.phase === "loading") current.ready.reject(err);
    if (current.phase === "running") current.active.reject(err);
  }

  #rejectPending(err) {
    const pending = this.#pending;
    this.#pending = null;
    pending?.reject(err);
  }

  #emitState() {
    try {
      this.#onStateChange(this.state);
    } catch (err) {
      reportObserverError(err);
    }
  }
}

const IDLE = Object.freeze({ phase: "idle" });
const DISPOSED = Object.freeze({ phase: "disposed" });

function deferred() {
  const settlement = {};
  settlement.promise = new Promise((resolve, reject) => {
    settlement.resolve = resolve;
    settlement.reject = reject;
  });
  return settlement;
}

function disposedError() {
  return new ValidationCanceledError("Validation worker disposed");
}

function defaultWorkerFactory(url) {
  return new Worker(url, { type: "module" });
}

function normalizeLimits(value) {
  if (!value || typeof value !== "object" || Array.isArray(value)) {
    throw new TypeError("Invalid validation limit catalog");
  }
  const limits = {
    maxXMLBytes: positiveSafeInteger(value.maxXMLBytes),
    maxFormattedXMLBytes: positiveSafeInteger(value.maxFormattedXMLBytes),
    maxXSDBytes: positiveSafeInteger(value.maxXSDBytes),
    maxValidationErrors: positiveSafeInteger(value.maxValidationErrors),
  };
  return Object.freeze(limits);
}

function positiveSafeInteger(value) {
  if (!Number.isSafeInteger(value) || value <= 0) {
    throw new TypeError("Invalid validation limit catalog");
  }
  return value;
}

function nonEmptyString(value) {
  return typeof value === "string" && value !== "" ? value : "";
}

function asError(value) {
  return value instanceof Error ? value : new Error(String(value));
}

function requestLimitError(xml, xsd, limits) {
  if (limits === null) return null;
  if (exceedsUTF8Bytes(xml, limits.maxXMLBytes)) {
    return new RangeError(`XML exceeds ${formatByteLimit(limits.maxXMLBytes)} limit`);
  }
  if (exceedsUTF8Bytes(xsd, limits.maxXSDBytes)) {
    return new RangeError(`XSD exceeds ${formatByteLimit(limits.maxXSDBytes)} limit`);
  }
  return null;
}

export function exceedsUTF8Bytes(text, limit) {
  if (text.length > limit) return true;

  let bytes = 0;
  for (let index = 0; index < text.length; index++) {
    const unit = text.charCodeAt(index);
    if (unit <= 0x7f) {
      bytes++;
    } else if (unit <= 0x7ff) {
      bytes += 2;
    } else if (unit >= 0xd800 && unit <= 0xdbff && index + 1 < text.length &&
        text.charCodeAt(index + 1) >= 0xdc00 && text.charCodeAt(index + 1) <= 0xdfff) {
      bytes += 4;
      index++;
    } else {
      bytes += 3;
    }
    if (bytes > limit) return true;
  }
  return false;
}

export function formatByteLimit(bytes) {
  const mebibyte = 1024 * 1024;
  const kibibyte = 1024;
  if (bytes % mebibyte === 0) return `${bytes / mebibyte} MiB`;
  if (bytes % kibibyte === 0) return `${bytes / kibibyte} KiB`;
  return `${bytes} bytes`;
}

function reportObserverError(value) {
  const err = asError(value);
  queueMicrotask(() => {
    if (typeof globalThis.reportError === "function") {
      globalThis.reportError(err);
      return;
    }
    console.error(err);
  });
}
