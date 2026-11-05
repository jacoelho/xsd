import "../wasm_exec.js";
import { runValidationFlow } from "./validation-flow.js";
import { startRuntime } from "./xsd-runtime.js";

const initialization = startRuntime({
  createGo: () => new Go(),
  instantiate,
  globals: globalThis,
  post: (message) => self.postMessage(message),
});

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

async function instantiate(importObject) {
  if (WebAssembly.instantiateStreaming) {
    try {
      const result = await WebAssembly.instantiateStreaming(fetch("../xsd.wasm"), importObject);
      return result.instance;
    } catch (_) {
      // A server with the wrong MIME type requires byte instantiation.
    }
  }
  const response = await fetch("../xsd.wasm");
  if (!response.ok) throw new Error(`WASM fetch failed: HTTP ${response.status}`);
  const bytes = await response.arrayBuffer();
  const result = await WebAssembly.instantiate(bytes, importObject);
  return result.instance;
}

function isValidationRequest(message) {
  return message && typeof message === "object" && !Array.isArray(message) &&
    message.type === "validate" && Number.isSafeInteger(message.requestId) &&
    typeof message.xml === "string" && typeof message.xsd === "string";
}
