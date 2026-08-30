export function runValidationFlow(xml, xsd, { validateXML, formatXML }) {
  const result = validationResponse(() => validateXML(xml, xsd));
  if (result.status !== "valid") {
    return { result, xml };
  }

  try {
    const formatted = parseResponse(formatXML(xml));
    if (isFormatResponse(formatted) && formatted.status === "ok") {
      return { result, xml: formatted.xml };
    }
  } catch (_) {
    // Formatting is optional after successful validation.
  }
  return { result, xml };
}

export function isValidationFlow(flow) {
  return flow && typeof flow === "object" && !Array.isArray(flow) &&
    typeof flow.xml === "string" && isValidationResponse(flow.result);
}

function validationResponse(validate) {
  let result;
  try {
    result = parseResponse(validate());
  } catch (err) {
    return { status: "error", error: `Invalid WASM response: ${err}` };
  }
  if (isValidationResponse(result)) {
    return result;
  }
  return { status: "error", error: "Invalid WASM validation response" };
}

function isValidationResponse(result) {
  if (!result || typeof result !== "object" || Array.isArray(result)) {
    return false;
  }
  switch (result.status) {
    case "valid":
      return !Object.hasOwn(result, "error") && !Object.hasOwn(result, "errors");
    case "invalid":
      return !Object.hasOwn(result, "error") && Array.isArray(result.errors) &&
        result.errors.length > 0 && result.errors.every(isDiagnostic);
    case "error":
      return typeof result.error === "string" && result.error !== "" &&
        !Object.hasOwn(result, "errors");
    default:
      return false;
  }
}

function isDiagnostic(diagnostic) {
  return diagnostic && typeof diagnostic === "object" &&
    !Array.isArray(diagnostic) && typeof diagnostic.message === "string" &&
    diagnostic.message !== "";
}

function isFormatResponse(result) {
  if (!result || typeof result !== "object" || Array.isArray(result)) {
    return false;
  }
  if (result.status === "ok") {
    return typeof result.xml === "string" && !Object.hasOwn(result, "error");
  }
  return result.status === "error" && typeof result.error === "string" &&
    result.error !== "" && !Object.hasOwn(result, "xml");
}

function parseResponse(value) {
  if (typeof value !== "string") {
    return value && typeof value === "object" ? value : {};
  }
  return JSON.parse(value || "{}");
}
