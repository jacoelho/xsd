import "../wasm_exec.js";
import { runValidationFlow } from "./validation-flow.js";

const initialization = initialize();

self.onmessage = async (event) => {
  const message = event.data;
  if (!isValidationRequest(message)) {
    self.postMessage({ type: "fatal", error: "Invalid validation request" });
    return;
  }

  try {
    const api = await initialization;
    const flow = runValidationFlow(message.xml, message.xsd, api);
    self.postMessage({ type: "result", requestId: message.requestId, flow });
  } catch (err) {
    self.postMessage({ type: "failure", requestId: message.requestId, error: String(err) });
  }
};

async function initialize() {
  try {
    const go = new Go();
    const instance = await instantiate(go);
    void go.run(instance).catch((err) => {
      self.postMessage({ type: "fatal", error: String(err) });
    });

    const limits = JSON.parse(globalThis.xsdLimits || "null");
    const api = Object.freeze({
      validateXML: globalThis.validateXML,
      formatXML: globalThis.formatXML,
    });
    if (typeof api.validateXML !== "function" || typeof api.formatXML !== "function") {
      throw new Error("WASM API did not initialize");
    }
    self.postMessage({ type: "ready", limits });
    return api;
  } catch (err) {
    self.postMessage({ type: "fatal", error: String(err) });
    throw err;
  }
}

async function instantiate(go) {
  if (WebAssembly.instantiateStreaming) {
    try {
      const result = await WebAssembly.instantiateStreaming(fetch("../xsd.wasm"), go.importObject);
      return result.instance;
    } catch (_) {
      // A server with the wrong MIME type requires byte instantiation.
    }
  }
  const response = await fetch("../xsd.wasm");
  if (!response.ok) throw new Error(`WASM fetch failed: HTTP ${response.status}`);
  const bytes = await response.arrayBuffer();
  const result = await WebAssembly.instantiate(bytes, go.importObject);
  return result.instance;
}

function isValidationRequest(message) {
  return message && typeof message === "object" && !Array.isArray(message) &&
    message.type === "validate" && Number.isSafeInteger(message.requestId) &&
    typeof message.xml === "string" && typeof message.xsd === "string";
}
