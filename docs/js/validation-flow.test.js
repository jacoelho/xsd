import assert from "node:assert/strict";
import test from "node:test";

import { runValidationFlow } from "./validation-flow.js";

test("validates the exact raw XML once before formatting", () => {
  const calls = [];
  const xml = "<root><value> raw </value></root>";

  const flow = runValidationFlow(xml, "schema", {
    validateXML(input, xsd) {
      calls.push(["validate", input, xsd]);
      return JSON.stringify({ valid: true });
    },
    formatXML(input) {
      calls.push(["format", input]);
      return JSON.stringify({ xml: "<root>\n  <value> raw </value>\n</root>" });
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
    validateXML: () => JSON.stringify({ valid: false, errors: [{ message: "unclosed element" }] }),
    formatXML() {
      formatCalls++;
      return JSON.stringify({ xml: "changed" });
    },
  });

  assert.equal(formatCalls, 0);
  assert.equal(flow.xml, "<root>");
  assert.equal(flow.result.valid, false);
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

test("uses successful formatted output", () => {
  const flow = runValidationFlow("<root/>", "schema", {
    validateXML: () => ({ valid: true }),
    formatXML: () => ({ xml: "<root />" }),
  });

  assert.deepEqual(flow, { result: { valid: true }, xml: "<root />" });
});

test("preserves valid input when formatting fails", () => {
  const xml = "<root/>";
  for (const formatXML of [
    () => JSON.stringify({ error: "format failed" }),
    () => {
      throw new Error("format failed");
    },
  ]) {
    const flow = runValidationFlow(xml, "schema", {
      validateXML: () => JSON.stringify({ valid: true }),
      formatXML,
    });
    assert.equal(flow.xml, xml);
    assert.equal(flow.result.valid, true);
  }
});
