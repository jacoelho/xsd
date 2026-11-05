# syntax=docker/dockerfile:1.7@sha256:a57df69d0ea827fb7266491f2813635de6f17269be881f696fbfdf2d83dda33e
# Pinned XSD 1.0 oracle image: Xerces-J, .NET System.Xml, Python xmlschema, libxml2.
# Verdicts are version-dependent; change a pin only together with re-recorded oracle metadata.

FROM eclipse-temurin:21.0.13_9-jdk-jammy@sha256:4b1a74fbffbe6b4e7b6f0171a472478282fb463790a51edf10e465d578f22b69 AS xerces
ADD --chmod=0644 --checksum=sha256:6fc991829af1708d15aea50c66f0beadcd2cfeb6968e0b2f55c1b0909883fe16 \
    https://repo1.maven.org/maven2/xerces/xercesImpl/2.12.2/xercesImpl-2.12.2.jar /out/xercesImpl.jar
COPY xerces/XercesOracle.java /src/
RUN javac --release 21 -cp /out/xercesImpl.jar -d /out /src/XercesOracle.java

FROM mcr.microsoft.com/dotnet/sdk:10.0.403-noble@sha256:de4fe90a98845d75910b9769662465c84de1a934665c431279d1382b2bddc02b AS dotnet
COPY dotnet/ /src/
RUN dotnet publish /src/DotnetOracle.csproj -c Release -o /out --nologo

FROM mcr.microsoft.com/dotnet/runtime:10.0.14-noble@sha256:a3f9f81954b9c810af2ed18849716c8dc74a4707bd0907fdbd69be81554c0f30 AS dotnet-runtime

# Jammy ships libxml2 2.9.13, the version recorded in tests/manifest.json oracle metadata.
FROM eclipse-temurin:21.0.13_9-jre-jammy@sha256:1eac8b8fee11ca7d6725a48bc5d41992ed1561504ebae25490d2d062847e8269
# jammy-updates keeps only the newest package versions and ports.ubuntu.com has no snapshot
# service, so these pins fail once superseded. To update, read the candidates with
#   docker run --rm <this base> sh -c 'apt-get update -qq; apt-cache policy libxml2-utils python3.10 libicu70'
# and re-record oracle metadata if the libxml2 version changes.
RUN apt-get update \
 && apt-get install -y --no-install-recommends \
      libxml2-utils=2.9.13+dfsg-1ubuntu0.14 \
      libxml2=2.9.13+dfsg-1ubuntu0.14 \
      python3.10=3.10.12-1~22.04.18 \
      libicu70=70.1-2 \
 && rm -rf /var/lib/apt/lists/*

COPY --from=dotnet-runtime /usr/share/dotnet /usr/share/dotnet
RUN ln -s /usr/share/dotnet/dotnet /usr/local/bin/dotnet
COPY --from=dotnet /out /opt/oracles/dotnet
COPY --from=xerces /out /opt/oracles/xerces

ADD --checksum=sha256:cf5c970a30f6ebcb3da35260e694704cc0b6794d8ace46125bb2a95e38ed9307 \
    https://files.pythonhosted.org/packages/py3/x/xmlschema/xmlschema-4.3.2-py3-none-any.whl /tmp/wheels/
ADD --checksum=sha256:5ec8d7bb44b16c98414a6a95e20cb150f3cbfc3cac65edc14773288c7a937376 \
    https://files.pythonhosted.org/packages/py3/e/elementpath/elementpath-5.1.4-py3-none-any.whl /tmp/wheels/
# Pure-Python wheels unpack directly; no installer or index access is needed.
RUN for w in /tmp/wheels/*.whl; do python3.10 -m zipfile -e "$w" /opt/oracles/xmlschema/lib; done \
 && rm -rf /tmp/wheels
COPY xmlschema/oracle.py /opt/oracles/xmlschema/
COPY libxml2/oracle.sh /opt/oracles/libxml2/
COPY run-all.sh /opt/oracles/

ENV HOME=/tmp DOTNET_CLI_TELEMETRY_OPTOUT=1 DOTNET_NOLOGO=1 DOTNET_ROOT=/usr/share/dotnet
WORKDIR /tmp
ENTRYPOINT ["/opt/oracles/run-all.sh"]
