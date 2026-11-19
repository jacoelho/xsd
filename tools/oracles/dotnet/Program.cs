using System;
using System.IO;
using System.Linq;
using System.Text.RegularExpressions;
using System.Xml;
using System.Xml.Schema;

// Usage: DotnetOracle INSTANCE SCHEMA...; INSTANCE is "" for a schema-only run.
// Prints "version<TAB>verdict<TAB>message".
static class Program
{
    // Schema and entity acquisition stays inside the mounted closure: a reference that is not a
    // local file under XSD_ORACLE_ROOT blocks the run instead of yielding a validity result.
    sealed class ClosureOnlyResolver : XmlUrlResolver
    {
        readonly string root;
        public string? Blocked;

        public ClosureOnlyResolver(string root) => this.root = root;

        bool InsideRoot(Uri uri)
        {
            if (!uri.IsFile || uri.IsUnc) return false;
            var path = Canonical(uri.LocalPath);
            return path is not null && path.StartsWith(root + "/", StringComparison.Ordinal) && File.Exists(path);
        }

        // Resolves symbolic links in every component so a link cannot leave the root.
        static string? Canonical(string path)
        {
            var full = Path.GetFullPath(path);
            var current = Path.GetPathRoot(full)!;
            foreach (var part in full.Substring(current.Length).Split('/', StringSplitOptions.RemoveEmptyEntries))
            {
                current = Path.Combine(current, part);
                FileSystemInfo info = Directory.Exists(current) ? new DirectoryInfo(current) : new FileInfo(current);
                if (info.LinkTarget is not null)
                {
                    var target = info.ResolveLinkTarget(true);
                    if (target is null) return null;
                    current = target.FullName;
                }
            }
            return current;
        }

        public override object? GetEntity(Uri absoluteUri, string? role, Type? ofObjectToReturn)
        {
            if (!InsideRoot(absoluteUri))
            {
                Blocked ??= absoluteUri.ToString();
                throw new XmlException($"acquisition blocked: {absoluteUri}");
            }
            return base.GetEntity(absoluteUri, role, ofObjectToReturn);
        }
    }

    static bool ReportBlocked(ClosureOnlyResolver resolver)
    {
        if (resolver.Blocked is null) return false;
        Emit("blocked", "acquisition blocked: " + resolver.Blocked);
        return true;
    }

    sealed class Diagnostics
    {
        public string? Error;
        public string? Warning;

        public void Record(object? sender, ValidationEventArgs e)
        {
            if (e.Severity == XmlSeverityType.Error) Error ??= Describe(e.Exception);
            else Warning ??= Describe(e.Exception);
        }

        // The first warning often names the root cause, such as an unloadable include.
        public string With(string? error)
        {
            if (Warning is null) return error ?? "";
            return error is null ? "warning: " + Warning : error + " | warning: " + Warning;
        }
    }

    static string Describe(XmlSchemaException e)
    {
        var where = e.SourceUri is null ? "" : Path.GetFileName(e.SourceUri);
        return $"{where}:{e.LineNumber}:{e.LinePosition}: {e.Message}";
    }

    static void Emit(string verdict, string message) =>
        Console.WriteLine($".NET {Environment.Version}\t{verdict}\t{Visible(RootRelative(message.Trim()))}");

    // Paths print relative to XSD_ORACLE_ROOT so a row is identical on every machine. File
    // URIs percent-encode the root (RFC 3986 section 2.1), so its encoded form is removed too.
    static string RootRelative(string s)
    {
        var root = Environment.GetEnvironmentVariable("XSD_ORACLE_ROOT")!;
        return StripRoot(StripRoot(s, string.Join("/", root.Split('/').Select(Uri.EscapeDataString))), root);
    }

    static string StripRoot(string s, string root)
    {
        return s.Replace("file://" + root + "/", "")
            .Replace("file:" + root + "/", "")
            .Replace(root + "/", "")
            .Replace(root, ".");
    }

    // Keeps CR, LF and TAB distinguishable in a one-line row: they are the discriminator in
    // end-of-line and whitespace cases.
    static string Visible(string s) =>
        s.Replace("\r", "\u240D").Replace("\n", "\u240A").Replace("\t", "\u2409");

    static int Main(string[] args)
    {
        var root = Environment.GetEnvironmentVariable("XSD_ORACLE_ROOT");
        if (args.Length < 2 || string.IsNullOrEmpty(root))
        {
            Console.Error.WriteLine("usage: XSD_ORACLE_ROOT=DIR DotnetOracle INSTANCE SCHEMA...");
            return 2;
        }
        var resolver = new ClosureOnlyResolver(root);
        var schemaDiag = new Diagnostics();
        var set = new XmlSchemaSet { XmlResolver = resolver };
        set.ValidationEventHandler += schemaDiag.Record;
        try
        {
            var settings = new XmlReaderSettings { DtdProcessing = DtdProcessing.Parse, XmlResolver = resolver };
            for (var i = 1; i < args.Length; i++)
            {
                using var reader = XmlReader.Create(args[i], settings);
                set.Add(null, reader);
            }
            set.Compile();
        }
        catch (XmlSchemaException e)
        {
            schemaDiag.Error ??= Describe(e);
        }
        catch (XmlException e)
        {
            schemaDiag.Error ??= e.Message;
        }
        catch (Exception e)
        {
            if (!ReportBlocked(resolver)) Emit("error", e.ToString());
            return 0;
        }
        if (ReportBlocked(resolver)) return 0;
        if (schemaDiag.Error is not null)
        {
            Emit("schema-invalid", schemaDiag.With(schemaDiag.Error));
            return 0;
        }
        if (args[0].Length == 0)
        {
            Emit("schema-valid", schemaDiag.With(null));
            return 0;
        }

        var instanceDiag = new Diagnostics();
        var instanceSettings = new XmlReaderSettings
        {
            ValidationType = ValidationType.Schema,
            Schemas = set,
            DtdProcessing = DtdProcessing.Parse,
            XmlResolver = resolver,
        };
        // Without warnings an undeclared validation root is silently assessed laxly.
        instanceSettings.ValidationFlags |= XmlSchemaValidationFlags.ReportValidationWarnings;
        instanceSettings.ValidationEventHandler += instanceDiag.Record;
        var rootAssessed = false;
        try
        {
            using var reader = XmlReader.Create(args[0], instanceSettings);
            while (reader.Read())
            {
                if (reader.Depth == 0 && reader.NodeType == XmlNodeType.Element)
                {
                    // A root with neither a declaration nor a type is assessed laxly;
                    // the warning text is not a verdict, the absent schema info is.
                    rootAssessed = reader.SchemaInfo?.SchemaElement is not null || reader.SchemaInfo?.SchemaType is not null;
                }
            }
        }
        catch (XmlSchemaException e)
        {
            if (!ReportBlocked(resolver)) Emit("invalid", instanceDiag.With(instanceDiag.Error ?? Describe(e)));
            return 0;
        }
        catch (XmlException e)
        {
            if (!ReportBlocked(resolver)) Emit("invalid", instanceDiag.With(instanceDiag.Error ?? e.Message));
            return 0;
        }
        catch (Exception e)
        {
            if (!ReportBlocked(resolver)) Emit("error", e.ToString());
            return 0;
        }
        if (ReportBlocked(resolver)) return 0;
        if (instanceDiag.Error is not null) Emit("invalid", instanceDiag.With(instanceDiag.Error));
        else Emit(rootAssessed ? "valid" : "unassessed", instanceDiag.With(null));
        return 0;
    }
}
