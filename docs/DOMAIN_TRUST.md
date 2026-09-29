# Domain trust checklist (mftools.xyz / genesisrk.mftools.xyz)

App-side hardening (security headers, CSP, scoped CORS, `security.txt`, `robots.txt`)
ships with GenesisRK. DNS / registrar items below must be done in **IONOS** (or your DNS host).

## Do in IONOS DNS / email

1. **DMARC** — upgrade from `p=none` once mail is verified:
   - Current: `"v=DMARC1; p=none;"`
   - Next: `"v=DMARC1; p=quarantine; rua=mailto:ala.eltai@suse.com; pct=25;"`
   - Later: `p=reject` when reports look clean.

2. **CAA** — restrict who can issue certs for the zone:
   ```
   mftools.xyz.  CAA 0 issue "digicert.com"
   mftools.xyz.  CAA 0 issuewild "digicert.com"
   mftools.xyz.  CAA 0 iodef "mailto:ala.eltai@suse.com"
   ```
   (Adjust issuer if you move off DigiCert / Azure-managed certs.)

3. **HTTPS on apex** — fix `https://mftools.xyz` (TLS error today). Either:
   - serve a simple landing that links to GenesisRK, or
   - HTTP→HTTPS redirect only after a valid cert exists for the apex.

4. **CDN / WAF (optional)** — put Cloudflare (orange cloud) in front of
   `genesisrk` CNAME if Azure shared-IP reputation stays noisy.

## Public reputation (re-scan after deploy)

- https://transparencyreport.google.com/safe-browsing/search?url=https://genesisrk.mftools.xyz
- https://urlscan.io/search/#domain%3Agenesisrk.mftools.xyz
- https://www.virustotal.com/gui/url/https://genesisrk.mftools.xyz
- https://www.ssllabs.com/ssltest/analyze.html?d=genesisrk.mftools.xyz
- https://securityheaders.com/?q=genesisrk.mftools.xyz&followRedirects=on
- https://mxtoolbox.com/SuperTool.aspx?action=blacklist%3agenesisrk.mftools.xyz

## False-positive reports

- Google Safe Browsing review via the transparency report
- Microsoft SmartScreen: https://www.microsoft.com/en-us/wdsi/support/report-unsafe-site-guest
- Your ISP’s phishing/abuse false-positive form
