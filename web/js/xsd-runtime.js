// startRuntime owns the Go WASM runtime lifecycle of the validation worker:
// instantiate, run, publish readiness, and report runtime loss. The Go program
// registers its API before it blocks, so the globals are read right after
// run() starts. Any settlement of run() means the program is gone and every
// later call would fail, so it is fatal. At most one fatal message is posted
// per runtime; a startup failure and the run settlement it causes collapse
// into the first.
export function startRuntime({ createGo, instantiate, globals, post }) {
  let failed = false;
  const fail = (err) => {
    if (failed) return;
    failed = true;
    post({ type: "fatal", error: String(err) });
  };

  return (async () => {
    try {
      const go = createGo();
      const instance = await instantiate(go.importObject);
      go.run(instance).then(
        () => fail("Go runtime exited"),
        (err) => fail(err),
      );

      const limits = JSON.parse(globals.xsdLimits || "null");
      const api = Object.freeze({
        validateXML: globals.validateXML,
        formatXML: globals.formatXML,
      });
      if (typeof api.validateXML !== "function" || typeof api.formatXML !== "function") {
        throw new Error("WASM API did not initialize");
      }
      post({ type: "ready", limits });
      return api;
    } catch (err) {
      fail(err);
      throw err;
    }
  })();
}
