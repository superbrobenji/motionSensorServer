<!-- SPDX-License-Identifier: GPL-3.0-or-later -->

# Security Policy

## Supported Versions

Only the latest commit on `main` receives security fixes.

## Reporting a Vulnerability

**Do not open a public GitHub issue for security vulnerabilities.**

Open a [GitHub Security Advisory](../../security/advisories/new) (private
disclosure), or email the maintainer at the address listed on their GitHub
profile.

Include:
- A clear description of the vulnerability
- Steps to reproduce
- Potential impact
- Any suggested remediation

You will receive an acknowledgement within 72 hours. If the vulnerability is
confirmed, a fix will be issued as soon as practical and credited to the
reporter unless anonymity is requested.

## Out of Scope

- Vulnerabilities in upstream dependencies (Kafka, Go stdlib, React Router)
  should be reported to those projects.
- Physical hardware attacks (UART/USB sniffing on ESP32 developer boards) are
  by design accessible and outside the scope of this policy.
