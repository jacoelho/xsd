import java.io.File;
import java.io.IOException;
import java.net.URL;
import java.net.URLDecoder;
import java.nio.charset.StandardCharsets;
import javax.xml.transform.stream.StreamSource;
import javax.xml.validation.Schema;
import javax.xml.validation.Validator;
import org.apache.xerces.impl.Version;
import org.apache.xerces.jaxp.validation.XMLSchemaFactory;
import org.w3c.dom.ls.LSInput;
import org.w3c.dom.ls.LSResourceResolver;
import org.xml.sax.ErrorHandler;
import org.xml.sax.SAXException;
import org.xml.sax.SAXParseException;

// Usage: XercesOracle INSTANCE SCHEMA...; INSTANCE is "" for a schema-only run.
// Prints "version<TAB>verdict<TAB>message".
public final class XercesOracle {
    private static final class Diagnostics implements ErrorHandler {
        String error;
        String warning;

        public void warning(SAXParseException e) {
            if (warning == null) warning = describe(e);
        }

        public void error(SAXParseException e) {
            if (error == null) error = describe(e);
        }

        public void fatalError(SAXParseException e) throws SAXException {
            error(e);
            throw e;
        }
    }

    private static final String ROOT = System.getenv("XSD_ORACLE_ROOT");
    private static String blocked;

    // Schema and entity acquisition stays inside the mounted closure: a reference that is not a
    // local file under XSD_ORACLE_ROOT blocks the run instead of yielding a validity result.
    // Locationless imports pass through unresolved.
    private static final LSResourceResolver CLOSURE_ONLY = (type, ns, publicId, systemId, baseURI) -> {
        if (systemId == null) return null;
        if (!insideRoot(baseURI, systemId)) {
            String ref = baseURI == null ? systemId : systemId + " (base " + baseURI + ")";
            if (blocked == null) blocked = ref;
            throw new IllegalStateException("acquisition blocked: " + ref);
        }
        return null;
    };

    // java.net.URL tolerates the unescaped spaces and non-ASCII that Xerces passes through;
    // getPath excludes the query and fragment.
    private static boolean insideRoot(String baseURI, String systemId) {
        try {
            URL url = baseURI == null ? new URL(systemId) : new URL(new URL(baseURI), systemId);
            if (!"file".equals(url.getProtocol())) return false;
            String host = url.getHost();
            if (host != null && !host.isEmpty() && !host.equals("localhost")) return false;
            String path = URLDecoder.decode(url.getPath().replace("+", "%2B"), StandardCharsets.UTF_8);
            File file = new File(path).getCanonicalFile();
            return file.isFile() && file.getPath().startsWith(ROOT + File.separator);
        } catch (IOException | IllegalArgumentException e) {
            return false;
        }
    }

    private static boolean reportBlocked() {
        if (blocked == null) return false;
        emit("blocked", "acquisition blocked: " + blocked);
        return true;
    }

    private static String describe(SAXParseException e) {
        String where = e.getSystemId() == null ? "" : new File(e.getSystemId()).getName();
        return where + ":" + e.getLineNumber() + ":" + e.getColumnNumber() + ": " + e.getMessage();
    }

    private static void emit(String verdict, String message) {
        String clean = message == null ? "" : message.replaceAll("\\s+", " ");
        System.out.println(Version.getVersion() + "\t" + verdict + "\t" + clean);
    }

    // The first warning often names the root cause, such as an unreadable include.
    private static String withWarning(String error, Diagnostics d) {
        if (d.warning == null) return error == null ? "" : error;
        return error == null ? "warning: " + d.warning : error + " | warning: " + d.warning;
    }

    public static void main(String[] args) {
        if (args.length < 2 || ROOT == null || ROOT.isEmpty()) {
            System.err.println("usage: XSD_ORACLE_ROOT=DIR XercesOracle INSTANCE SCHEMA...");
            System.exit(2);
        }
        StreamSource[] roots = new StreamSource[args.length - 1];
        for (int i = 1; i < args.length; i++) roots[i - 1] = new StreamSource(new File(args[i]));
        XMLSchemaFactory factory = new XMLSchemaFactory();
        Diagnostics schemaDiag = new Diagnostics();
        factory.setErrorHandler(schemaDiag);
        factory.setResourceResolver(CLOSURE_ONLY);
        Schema schema;
        try {
            schema = factory.newSchema(roots);
        } catch (SAXException e) {
            if (reportBlocked()) return;
            emit("schema-invalid", withWarning(schemaDiag.error != null ? schemaDiag.error : e.getMessage(), schemaDiag));
            return;
        } catch (RuntimeException e) {
            if (!reportBlocked()) emit("error", e.toString());
            return;
        }
        if (reportBlocked()) return;
        if (schemaDiag.error != null) {
            emit("schema-invalid", withWarning(schemaDiag.error, schemaDiag));
            return;
        }
        if (args[0].isEmpty()) {
            emit("schema-valid", withWarning(null, schemaDiag));
            return;
        }
        Validator validator = schema.newValidator();
        Diagnostics instanceDiag = new Diagnostics();
        validator.setErrorHandler(instanceDiag);
        validator.setResourceResolver(CLOSURE_ONLY);
        try {
            validator.validate(new StreamSource(new File(args[0])));
        } catch (SAXException e) {
            if (reportBlocked()) return;
            emit("invalid", withWarning(instanceDiag.error != null ? instanceDiag.error : e.getMessage(), instanceDiag));
            return;
        } catch (Exception e) {
            if (!reportBlocked()) emit("error", e.toString());
            return;
        }
        if (reportBlocked()) return;
        if (instanceDiag.error != null) {
            emit("invalid", withWarning(instanceDiag.error, instanceDiag));
        } else {
            emit("valid", withWarning(null, instanceDiag));
        }
    }
}
