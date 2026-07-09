export function runValidationFlow(xml, xsd, { validateXML, formatXML }) {
  const result = validationResponse(() => validateXML(xml, xsd));
  if (result.error || !result.valid) {
    return { result, xml };
  }

  try {
    const formatted = parseResponse(formatXML(xml));
    if (!formatted.error && typeof formatted.xml === "string") {
      return { result, xml: formatted.xml };
    }
  } catch (_) {
    // Formatting is optional after successful validation.
  }
  return { result, xml };
}

function validationResponse(validate) {
  let result;
  try {
    result = parseResponse(validate());
  } catch (err) {
    return { error: `Validation failed: ${err}` };
  }
  if (result.error || typeof result.valid === "boolean") {
    return result;
  }
  return { error: "Invalid WASM validation response" };
}

function parseResponse(value) {
  if (typeof value !== "string") {
    return value && typeof value === "object" ? value : {};
  }
  try {
    return JSON.parse(value || "{}");
  } catch (err) {
    return { error: `Invalid WASM response: ${err}` };
  }
}
