import { chromium } from "@playwright/test";

// Run against `make web`; initialization is excluded, worker round trips and
// the normal formatting flow are included. Each sample has one active request.
const schema = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root"><xs:complexType><xs:sequence>
    <xs:element name="v" type="xs:int"/>
  </xs:sequence></xs:complexType></xs:element>
</xs:schema>`;
const browser = await chromium.launch();
try {
  const page = await browser.newPage();
  await page.goto(process.env.XSD_BENCH_URL || "http://127.0.0.1:8765");
  const result = await page.evaluate(async (xsd) => {
    const { ValidationWorkerClient } = await import("/js/validation-worker.js");
    const client = new ValidationWorkerClient("/js/xsd-worker.js");
    const edits = 100;
    const samples = [];
    try {
      await client.start();
      for (let sample = 0; sample < 10; sample++) {
        const modes = sample % 2 ? ["changed-schema", "same-schema"] : ["same-schema", "changed-schema"];
        for (const mode of modes) {
          await client.validate("<root><v>1</v></root>", xsd);
          const start = performance.now();
          for (let i = 0; i < edits; i++) {
            const source = mode === "changed-schema" && i % 2 === 0 ? xsd + "\n" : xsd;
            const flow = await client.validate(`<root><v>${1 + i % 2}</v></root>`, source);
            if (flow.result.status !== "valid") throw new Error(JSON.stringify(flow));
          }
          samples.push({ sample, mode, millisecondsPerEdit: (performance.now() - start) / edits });
        }
      }
      return { userAgent: navigator.userAgent, editsPerSample: edits, samples };
    } finally {
      client.dispose();
    }
  }, schema);
  console.log(JSON.stringify(result, null, 2));
} finally {
  await browser.close();
}
