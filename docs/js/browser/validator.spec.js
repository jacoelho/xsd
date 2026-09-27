import { expect, test } from "@playwright/test";

const schema = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root">
    <xs:complexType>
      <xs:sequence>
        <xs:element name="v" type="xs:int"/>
      </xs:sequence>
    </xs:complexType>
  </xs:element>
</xs:schema>`;

test("validates in a worker and renders valid and invalid outcomes", async ({ page }) => {
  const pageErrors = [];
  page.on("pageerror", (err) => pageErrors.push(err.message));

  await page.goto("/");
  const validate = page.locator("#validate-button");
  await expect(validate).toBeEnabled({ timeout: 30_000 });
  await expect(page.locator("#xsd-files")).toBeEnabled();
  await expect(page.locator("#xml-file")).toBeEnabled();

  await expect.poll(() => page.evaluate(() => ({
    formatXML: typeof window.formatXML,
    validateXML: typeof window.validateXML,
    xsdLimits: typeof window.xsdLimits,
  }))).toEqual({ formatXML: "undefined", validateXML: "undefined", xsdLimits: "undefined" });

  await page.locator("#xsd-editor").fill(schema);
  await page.locator("#xml-editor").fill("<root><v>1</v></root>");
  await validate.click();
  await expect(page.locator("#result-title")).toHaveText("Valid XML");
  await expect(page.locator("#xml-editor")).toHaveValue("<root>\n  <v>1</v>\n</root>");

  await page.locator("#xml-editor").fill("<root><v>x</v></root>");
  await validate.click();
  await expect(page.locator("#result-title")).toHaveText("1 validation error");
  await expect(page.locator("#result-body tbody tr")).toHaveCount(1);
  await expect(page.locator("#result-body")).toContainText("validation.facet");
  expect(pageErrors).toEqual([]);
});

test("changed XSD replaces the worker's previously compiled schema", async ({ page }) => {
  await page.goto("/");
  const validate = page.locator("#validate-button");
  await expect(validate).toBeEnabled({ timeout: 30_000 });
  await page.locator("#xsd-editor").fill(schema);
  await page.locator("#xml-editor").fill("<root><v>2</v></root>");
  await validate.click();
  await expect(page.locator("#result-title")).toHaveText("Valid XML");

  await page.locator("#xsd-editor").fill(schema.replace("xs:int", "xs:boolean"));
  await validate.click();
  await expect(page.locator("#result-title")).toHaveText("1 validation error");
  await expect(page.locator("#result-body")).toContainText("validation.facet");
  await page.locator("#xml-editor").fill("<root><v>true</v></root>");
  await validate.click();
  await expect(page.locator("#result-title")).toHaveText("Valid XML");
});

test("malformed XML renders assessment diagnostics without formatting", async ({ page }) => {
  await page.goto("/");
  const validate = page.locator("#validate-button");
  await expect(validate).toBeEnabled({ timeout: 30_000 });
  await page.locator("#xsd-editor").fill(schema);
  const malformed = "<root><v>1</root>";
  await page.locator("#xml-editor").fill(malformed);
  await validate.click();
  await expect(page.locator("#result-title")).toHaveText("Validation failed");
  await expect(page.locator("#result-body tbody tr")).toHaveCount(1);
  await expect(page.locator("#result-body")).toContainText("validation.xml");
  await expect(page.locator("#xml-editor")).toHaveValue(malformed);
});

test("timeout replaces a warmed WASM worker and the next schema is assessed", async ({ page }) => {
  await page.goto("/");
  await expect(page.locator("#validate-button")).toBeEnabled({ timeout: 30_000 });
  await page.clock.install();
  const initial = await page.evaluate(async (xsd) => {
    const { ValidationWorkerClient } = await import("/js/validation-worker.js");
    window.dropValidation = false;
    window.workerGenerations = 0;
    window.timeoutClient = new ValidationWorkerClient("/js/xsd-worker.js", {
      workerFactory(url) {
        window.workerGenerations++;
        const worker = new Worker(url, { type: "module" });
        const post = worker.postMessage.bind(worker);
        worker.postMessage = (message, ...transfer) => {
          // Lose one request deterministically; timing must not depend on
          // how quickly this machine compiles or executes WASM.
          if (window.dropValidation && message.type === "validate") {
            window.dropValidation = false;
            return;
          }
          post(message, ...transfer);
        };
        return worker;
      },
    });
    await window.timeoutClient.start();
    return (await window.timeoutClient.validate("<root><v>2</v></root>", xsd)).result.status;
  }, schema);
  expect(initial).toBe("valid");

  await page.evaluate((xsd) => {
    window.dropValidation = true;
    window.timeoutResult = window.timeoutClient.validate("<root><v>2</v></root>", xsd)
      .then(() => "unexpected success", (err) => err.name);
  }, schema);
  await page.clock.fastForward(30_001);
  const resumed = await page.evaluate(async (xsd) => {
    try {
      const timedOut = await window.timeoutResult;
      const flow = await window.timeoutClient.validate("<root><v>2</v></root>", xsd);
      return { timedOut, status: flow.result.status, generations: window.workerGenerations };
    } finally {
      window.timeoutClient.dispose();
    }
  }, schema.replace("xs:int", "xs:boolean"));
  expect(resumed).toEqual({ timedOut: "ValidationTimeoutError", status: "invalid", generations: 2 });
});

test("a completed file read invalidates validation of the previous input", async ({ page }) => {
  await page.addInitScript(() => {
    const read = File.prototype.arrayBuffer;
    File.prototype.arrayBuffer = function deferredArrayBuffer() {
      return new Promise((resolve, reject) => {
        window.releaseFileRead = () => read.call(this).then(resolve, reject);
      });
    };

    const post = Worker.prototype.postMessage;
    Worker.prototype.postMessage = function deferredValidation(message, ...transfer) {
      if (message?.type === "validate") {
        window.releaseValidation = () => post.call(this, message, ...transfer);
        return;
      }
      post.call(this, message, ...transfer);
    };
  });

  await page.goto("/");
  await expect(page.locator("#validate-button")).toBeEnabled({ timeout: 30_000 });
  await page.locator("#xsd-editor").fill(schema);
  await page.locator("#xml-editor").fill("<root><v>1</v></root>");

  await page.locator("#xml-file").setInputFiles({
    name: "replacement.xml",
    mimeType: "application/xml",
    buffer: Buffer.from("<root><v>2</v></root>"),
  });
  await expect.poll(() => page.evaluate(() => typeof window.releaseFileRead)).toBe("function");
  await page.locator("#validate-button").click();
  await expect.poll(() => page.evaluate(() => typeof window.releaseValidation)).toBe("function");

  await page.evaluate(() => window.releaseFileRead());
  await expect(page.locator("#xml-editor")).toHaveValue("<root><v>2</v></root>");
  await page.evaluate(() => window.releaseValidation());

  await expect(page.locator("#xml-editor")).toHaveValue("<root><v>2</v></root>");
  await expect(page.locator("#result-title")).toHaveText("Validation result");
});

test("manual edits and Clear supersede pending file reads", async ({ page }) => {
  await page.addInitScript(() => {
    const read = File.prototype.arrayBuffer;
    File.prototype.arrayBuffer = function deferredArrayBuffer() {
      return new Promise((resolve, reject) => {
        window.releaseFileRead = () => read.call(this).then(resolve, reject);
      });
    };
  });

  await page.goto("/");
  await expect(page.locator("#validate-button")).toBeEnabled({ timeout: 30_000 });

  await page.locator("#xml-file").setInputFiles({
    name: "stale.xml",
    mimeType: "application/xml",
    buffer: Buffer.from("<stale/>")
  });
  await expect.poll(() => page.evaluate(() => typeof window.releaseFileRead)).toBe("function");
  await page.locator("#xml-editor").fill("<manual/>");
  await page.evaluate(() => window.releaseFileRead());
  await expect(page.locator("#xml-editor")).toHaveValue("<manual/>");

  await page.locator("#xml-file").setInputFiles({
    name: "also-stale.xml",
    mimeType: "application/xml",
    buffer: Buffer.from("<also-stale/>")
  });
  await page.locator("#clear-button").click();
  await page.evaluate(() => window.releaseFileRead());
  await expect(page.locator("#xml-editor")).toHaveValue("");
});

test("oversized pasted input skips syntax and line rendering", async ({ page }) => {
  const pageErrors = [];
  page.on("pageerror", (err) => pageErrors.push(err.message));
  await page.goto("/");
  await expect(page.locator("#validate-button")).toBeEnabled({ timeout: 30_000 });

  await page.evaluate(() => {
    const editor = document.querySelector("#xml-editor");
    editor.value = "\n".repeat((2 * 1024 * 1024) + 1);
    editor.dispatchEvent(new Event("input", { bubbles: true }));
  });

  await expect(page.locator("#result-title")).toHaveText("XML input failed");
  await expect(page.locator("#xml-syntax")).toBeEmpty();
  await expect(page.locator("#xml-gutter > *")).toHaveCount(1);
  await expect(page.locator("#xml-editor")).toHaveClass(/plain-text/);
  expect(pageErrors).toEqual([]);
});

test("exact-size newline input has bounded line rendering", async ({ page }) => {
  await page.goto("/");
  await expect(page.locator("#validate-button")).toBeEnabled({ timeout: 30_000 });

  const length = await page.evaluate(() => {
    const editor = document.querySelector("#xml-editor");
    editor.value = "\n".repeat(2 * 1024 * 1024);
    editor.dispatchEvent(new Event("input", { bubbles: true }));
    return editor.value.length;
  });

  expect(length).toBe(2 * 1024 * 1024);
  await expect(page.locator("#result-title")).toHaveText("Validation result");
  await expect(page.locator("#xml-syntax")).toBeEmpty();
  await expect(page.locator("#xml-gutter > *")).toHaveCount(1);
  await expect(page.locator("#xml-editor")).toHaveClass(/plain-text/);
});
