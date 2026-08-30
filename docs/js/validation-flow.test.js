import assert from "node:assert/strict";
import test from "node:test";

import { runValidationFlow } from "./validation-flow.js";

test("validates the exact raw XML once before formatting", () => {
  const calls = [];
  const xml = "<root><value> raw </value></root>";

  const flow = runValidationFlow(xml, "schema", {
    validateXML(input, xsd) {
      calls.push(["validate", input, xsd]);
      return JSON.stringify({ status: "valid" });
    },
    formatXML(input) {
      calls.push(["format", input]);
      return JSON.stringify({ status: "ok", xml: "<root>\n  <value> raw </value>\n</root>" });
    },
  });

  assert.deepEqual(calls, [
    ["validate", xml, "schema"],
    ["format", xml],
  ]);
  assert.equal(flow.xml, "<root>\n  <value> raw </value>\n</root>");
});

test("does not format invalid XML", () => {
  let formatCalls = 0;
  const flow = runValidationFlow("<root>", "schema", {
    validateXML: () => JSON.stringify({ status: "invalid", errors: [{ message: "unclosed element" }] }),
    formatXML() {
      formatCalls++;
      return JSON.stringify({ status: "ok", xml: "changed" });
    },
  });

  assert.equal(formatCalls, 0);
  assert.equal(flow.xml, "<root>");
  assert.equal(flow.result.status, "invalid");
});

test("reports an invalid validation response", () => {
  const flow = runValidationFlow("<root/>", "schema", {
    validateXML: () => "not JSON",
    formatXML: () => {
      throw new Error("must not format");
    },
  });

  assert.match(flow.result.error, /^Invalid WASM response:/);
  assert.equal(flow.xml, "<root/>");
});

test("rejects contradictory and malformed validation result states", () => {
  for (const response of [
    { status: "invalid", errors: {} },
    { status: "invalid", errors: [] },
    { status: "valid", errors: [{ message: "contradiction" }] },
    { status: "valid", error: "contradiction" },
    { status: "error", error: "contradiction", errors: [] },
    { valid: true },
    null,
  ]) {
    const flow = runValidationFlow("<root/>", "schema", {
      validateXML: () => response,
      formatXML: () => {
        throw new Error("must not format");
      },
    });
    assert.deepEqual(flow.result, {
      status: "error",
      error: "Invalid WASM validation response",
    });
  }
});

test("uses successful formatted output", () => {
  const flow = runValidationFlow("<root/>", "schema", {
    validateXML: () => ({ status: "valid" }),
    formatXML: () => ({ status: "ok", xml: "<root />" }),
  });

  assert.deepEqual(flow, { result: { status: "valid" }, xml: "<root />" });
});

test("preserves valid input when formatting fails", () => {
  const xml = "<root/>";
  for (const formatXML of [
    () => JSON.stringify({ status: "error", error: "format failed" }),
    () => {
      throw new Error("format failed");
    },
  ]) {
    const flow = runValidationFlow(xml, "schema", {
      validateXML: () => JSON.stringify({ status: "valid" }),
      formatXML,
    });
    assert.equal(flow.xml, xml);
    assert.equal(flow.result.status, "valid");
  }
});
