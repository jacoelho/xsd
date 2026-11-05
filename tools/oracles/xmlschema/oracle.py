"""Usage: oracle.py INSTANCE SCHEMA...; INSTANCE is "" for a schema-only run.

Prints "version<TAB>verdict<TAB>message".
"""
import os
import re
import sys
import urllib.request
import warnings
from urllib.error import URLError
from xml.etree.ElementTree import ParseError

import xmlschema
from xmlschema.exceptions import XMLResourceError

ROOT = os.environ.get("XSD_ORACLE_ROOT", "")
blocked = []


def inside_root(req):
    if req.type != "file" or req.host not in ("", "localhost"):
        return False
    path = os.path.realpath(urllib.request.url2pathname(req.selector))
    return path.startswith(ROOT + os.sep) and os.path.isfile(path)


class ClosureOnly(urllib.request.BaseHandler):
    """Schema and entity acquisition stays inside the mounted closure: a reference
    that is not a local file under XSD_ORACLE_ROOT blocks the run instead of
    yielding a validity result."""

    def default_open(self, req):
        if not inside_root(req):
            blocked.append(req.full_url)
            raise URLError(f"acquisition blocked: {req.full_url}")
        return None


# Only the gate and file: handlers are installed, so no other scheme can open.
OPENER = urllib.request.OpenerDirector()
OPENER.add_handler(ClosureOnly())
OPENER.add_handler(urllib.request.FileHandler())


def report_blocked(caught):
    if not blocked:
        return False
    emit("blocked", f"acquisition blocked: {blocked[0]}", caught)
    return True


def emit(verdict, message, caught):
    message = re.sub(r"\s+", " ", str(message))
    # The first warning often names the root cause, such as an unreadable include.
    if caught:
        warning = re.sub(r"\s+", " ", str(caught[0].message))
        message = f"{message} | warning: {warning}" if message else f"warning: {warning}"
    print(f"xmlschema {xmlschema.__version__}\t{verdict}\t{message}")


def main(argv):
    if len(argv) < 2 or not ROOT:
        print("usage: XSD_ORACLE_ROOT=DIR oracle.py INSTANCE SCHEMA...", file=sys.stderr)
        return 2
    instance, roots = argv[0], argv[1:]
    with warnings.catch_warnings(record=True) as caught:
        warnings.simplefilter("always")
        try:
            # use_fallback=False: no bundled schema documents stand in for absent imports.
            schema = xmlschema.XMLSchema10(
                roots if len(roots) > 1 else roots[0], opener=OPENER, use_fallback=False
            )
        except (xmlschema.XMLSchemaParseError, xmlschema.XMLSchemaValidationError, ParseError) as e:
            if not report_blocked(caught):
                emit("schema-invalid", e, caught)
            return 0
        except Exception as e:  # provider failure, not a validity verdict
            if not report_blocked(caught):
                emit("error", f"{type(e).__name__}: {e}", caught)
            return 0
        if report_blocked(caught):
            return 0
        if not instance:
            emit("schema-valid", "", caught)
            return 0
        del caught[:]

        errors = []
        try:
            resource = xmlschema.XMLResource(instance, opener=OPENER)
            for error in schema.iter_errors(resource, use_location_hints=False):
                errors.append(error)
        except (ParseError, XMLResourceError) as e:
            if not report_blocked(caught):
                emit("invalid", f"not well-formed: {e}", caught)
            return 0
        except Exception as e:  # a terminal exception leaves the run incomplete even after yielded errors
            if not report_blocked(caught):
                emit("error", f"{type(e).__name__} after {len(errors)} validation error(s): {e}", caught)
            return 0
        if report_blocked(caught):
            return 0
        if errors:
            first = errors[0]
            emit("invalid", f"line {first.sourceline}: {first.reason} (path {first.path})", caught)
        else:
            emit("valid", "", caught)
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
