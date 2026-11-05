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

export class ValidationWorkerClient {
  #active = null;
  #disposed = false;
  #generation = 0;
  #initializationTimeout = null;
  #limits = null;
  #onStateChange;
  #pending = null;
  #phase = "idle";
  #ready = null;
  #rejectReady = null;
  #resolveReady = null;
  #sequence = 0;
  #timeout = null;
  #timeoutMs;
  #worker = null;
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

  get state() {
    return Object.freeze({
      phase: this.#phase,
      busy: this.#active !== null || this.#pending !== null,
      limits: this.#limits,
    });
  }

  start() {
    if (this.#disposed) return Promise.reject(new ValidationCanceledError("Validation worker disposed"));
    if (this.#phase === "ready" || this.#phase === "running") {
      return Promise.resolve(this.#limits);
    }
    if (this.#phase !== "loading") this.#spawn();
    return this.#ready;
  }

  validate(xml, xsd) {
    if (typeof xml !== "string" || typeof xsd !== "string") {
      return Promise.reject(new TypeError("xml and xsd must be strings"));
    }
    if (this.#disposed) return Promise.reject(new ValidationCanceledError("Validation worker disposed"));

    const limitError = requestLimitError(xml, xsd, this.#limits);
    if (limitError !== null) return Promise.reject(limitError);

    let resolve;
    let reject;
    const promise = new Promise((onResolve, onReject) => {
      resolve = onResolve;
      reject = onReject;
    });
    const request = { id: ++this.#sequence, xml, xsd, resolve, reject };

    if (this.#pending !== null) {
      this.#pending.reject(new ValidationSupersededError());
    }
    this.#pending = request;

    if (this.#phase === "idle" || this.#phase === "failed") this.#spawn();
    this.#drain();
    this.#emitState();
    return promise;
  }

  cancel(message) {
    if (this.#disposed) return Promise.resolve(null);
    const err = new ValidationCanceledError(message);
    this.#rejectRequests(err, true);
    this.#terminateWorker();
    this.#spawn();
    return this.#ready;
  }

  dispose() {
    if (this.#disposed) return;
    this.#disposed = true;
    this.#rejectRequests(new ValidationCanceledError("Validation worker disposed"), true);
    this.#terminateWorker();
    this.#phase = "disposed";
    this.#emitState();
  }

  #spawn() {
    this.#terminateWorker();
    this.#phase = "loading";
    this.#limits = null;
    this.#generation++;
    const generation = this.#generation;
    this.#ready = new Promise((resolve, reject) => {
      this.#resolveReady = resolve;
      this.#rejectReady = reject;
    });
    this.#ready.catch(() => {});

    try {
      const worker = this.#workerFactory(this.#workerURL);
      this.#worker = worker;
      worker.onmessage = (event) => this.#receive(generation, event.data);
      worker.onerror = (event) => {
        const message = event?.message || "Validation worker failed";
        this.#failWorker(generation, new Error(message));
      };
      this.#initializationTimeout = setTimeout(
        () => this.#initializationTimedOut(generation),
        this.#timeoutMs,
      );
    } catch (err) {
      this.#failWorker(generation, asError(err));
    }
    this.#emitState();
  }

  #receive(generation, message) {
    if (generation !== this.#generation || this.#disposed) return;
    if (!message || typeof message !== "object" || Array.isArray(message)) {
      this.#failWorker(generation, new Error("Invalid validation worker message"));
      return;
    }

    switch (message.type) {
      case "ready":
        this.#workerReady(generation, message.limits);
        return;
      case "result":
        this.#completeRequest(message.requestId, message.flow);
        return;
      case "failure":
        this.#rejectRequest(message.requestId, new Error(nonEmptyString(message.error) || "Validation failed"));
        return;
      case "fatal":
        this.#failWorker(generation, new Error(nonEmptyString(message.error) || "Validation worker failed"));
        return;
      default:
        this.#failWorker(generation, new Error("Invalid validation worker message"));
    }
  }

  #workerReady(generation, limits) {
    if (generation !== this.#generation || this.#phase !== "loading") {
      this.#failWorker(generation, new Error("Unexpected validation worker readiness"));
      return;
    }
    try {
      this.#limits = normalizeLimits(limits);
    } catch (err) {
      this.#failWorker(generation, asError(err));
      return;
    }
    clearTimeout(this.#initializationTimeout);
    this.#initializationTimeout = null;
    this.#phase = "ready";
    this.#drain();
    if (this.#phase === "failed") return;
    const resolve = this.#resolveReady;
    this.#resolveReady = null;
    this.#rejectReady = null;
    resolve(this.#limits);
    this.#emitState();
  }

  #drain() {
    if (this.#phase !== "ready" || this.#active !== null || this.#pending === null) return;
    const request = this.#pending;
    this.#pending = null;
    const limitError = requestLimitError(request.xml, request.xsd, this.#limits);
    if (limitError !== null) {
      request.reject(limitError);
      return;
    }
    this.#active = request;
    this.#phase = "running";
    try {
      this.#worker.postMessage({
        type: "validate",
        requestId: request.id,
        xml: request.xml,
        xsd: request.xsd,
      });
    } catch (err) {
      this.#failWorker(this.#generation, asError(err));
      return;
    }
    this.#timeout = setTimeout(() => this.#requestTimedOut(request.id), this.#timeoutMs);
    this.#emitState();
  }

  #completeRequest(requestId, flow) {
    if (!this.#matchesActive(requestId)) {
      this.#failWorker(this.#generation, new Error("Invalid validation worker request ID"));
      return;
    }
    if (!isValidationFlow(flow)) {
      this.#failWorker(this.#generation, new Error("Invalid validation worker result"));
      return;
    }
    const request = this.#takeActive();
    request.resolve(flow);
    this.#continueAfterRequest();
  }

  #rejectRequest(requestId, err) {
    if (!this.#matchesActive(requestId)) {
      this.#failWorker(this.#generation, new Error("Invalid validation worker request ID"));
      return;
    }
    const request = this.#takeActive();
    request.reject(err);
    this.#continueAfterRequest();
  }

  #requestTimedOut(requestId) {
    if (!this.#matchesActive(requestId)) return;
    const request = this.#takeActive();
    request.reject(new ValidationTimeoutError(this.#timeoutMs));
    this.#terminateWorker();
    this.#spawn();
  }

  #initializationTimedOut(generation) {
    if (generation !== this.#generation || this.#phase !== "loading") return;
    this.#failWorker(generation, new ValidationTimeoutError(this.#timeoutMs));
  }

  #continueAfterRequest() {
    this.#phase = "ready";
    this.#drain();
    this.#emitState();
  }

  #matchesActive(requestId) {
    return Number.isSafeInteger(requestId) && this.#active?.id === requestId;
  }

  #takeActive() {
    clearTimeout(this.#timeout);
    this.#timeout = null;
    const request = this.#active;
    this.#active = null;
    return request;
  }

  #failWorker(generation, err) {
    if (generation !== this.#generation || this.#disposed) return;
    const rejectReady = this.#rejectReady;
    this.#rejectReady = null;
    this.#resolveReady = null;
    this.#rejectRequests(err, false);
    this.#terminateWorker();
    this.#phase = "failed";
    if (rejectReady !== null) rejectReady(err);
    this.#emitState();
  }

  #rejectRequests(err, rejectReady) {
    clearTimeout(this.#timeout);
    this.#timeout = null;
    if (this.#active !== null) this.#active.reject(err);
    if (this.#pending !== null) this.#pending.reject(err);
    this.#active = null;
    this.#pending = null;
    if (rejectReady && this.#rejectReady !== null) this.#rejectReady(err);
    if (rejectReady) {
      this.#rejectReady = null;
      this.#resolveReady = null;
    }
  }

  #terminateWorker() {
    clearTimeout(this.#initializationTimeout);
    this.#initializationTimeout = null;
    clearTimeout(this.#timeout);
    this.#timeout = null;
    if (this.#worker === null) return;
    this.#worker.onmessage = null;
    this.#worker.onerror = null;
    this.#worker.terminate();
    this.#worker = null;
  }

  #emitState() {
    try {
      this.#onStateChange(this.state);
    } catch (err) {
      reportObserverError(err);
    }
  }
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
