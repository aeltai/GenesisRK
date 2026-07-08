#!/usr/bin/env bash
# Configure a custom domain (with managed TLS) for the Genesis Container App.
#
# Azure validates domain ownership before binding. Add the DNS records printed by
# this script, then run: ./custom-domain.sh bind
#
# Usage:
#   ./custom-domain.sh dns      # print required DNS records (default)
#   ./custom-domain.sh check    # verify DNS propagation
#   ./custom-domain.sh bind     # add hostname + managed certificate (DNS must be ready)
#   ./custom-domain.sh status   # show current custom domains

set -e
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

if [[ -f "$SCRIPT_DIR/.env" ]]; then
  set -a
  source "$SCRIPT_DIR/.env"
  set +a
fi

RESOURCE_GROUP_NAME="${RESOURCE_GROUP_NAME:-pse-aeltai-aks-rg25}"
CONTAINERAPP_NAME="${CONTAINERAPP_NAME:-genesis-app}"
CONTAINERAPP_ENV="${CONTAINERAPP_ENV:-genesis-env}"
CUSTOM_DOMAIN="${CUSTOM_DOMAIN:-genesisrk.mftools.xyz}"
AZURE_LOCATION="${AZURE_LOCATION:-westeurope}"

FQDN=$(az containerapp show --name "$CONTAINERAPP_NAME" --resource-group "$RESOURCE_GROUP_NAME" \
  --query "properties.configuration.ingress.fqdn" -o tsv)
VERIFICATION_ID=$(az containerapp env show --name "$CONTAINERAPP_ENV" --resource-group "$RESOURCE_GROUP_NAME" \
  --query "properties.customDomainConfiguration.customDomainVerificationId" -o tsv)

if [[ -z "$FQDN" || -z "$VERIFICATION_ID" ]]; then
  echo "Could not read Container App FQDN or domain verification ID."
  echo "Check CONTAINERAPP_NAME, CONTAINERAPP_ENV, and RESOURCE_GROUP_NAME in .env"
  exit 1
fi

# For zone mftools.xyz: host "genesisrk" -> genesisrk.mftools.xyz
SUBDOMAIN="${CUSTOM_DOMAIN%%.mftools.xyz}"
if [[ "$SUBDOMAIN" == "$CUSTOM_DOMAIN" ]]; then
  # Generic fallback: strip first label from hostname for TXT asuid.<sub>
  SUBDOMAIN="${CUSTOM_DOMAIN%%.*}"
fi

print_dns() {
  cat <<EOF
Add these DNS records in the mftools.xyz zone:

  Type   Name              Value
  ----   ----              -----
  CNAME  genesisrk         ${FQDN}
  TXT    asuid.genesisrk   ${VERIFICATION_ID}

After propagation (often 1–15 minutes), run:

  cd deploy/azure && ./custom-domain.sh bind

App URL when ready: https://${CUSTOM_DOMAIN}
Current Azure URL:  https://${FQDN}
EOF
}

check_dns() {
  local ok=1
  echo "Checking DNS for ${CUSTOM_DOMAIN}..."
  echo

  local cname
  cname=$(dig +short "CNAME" "$CUSTOM_DOMAIN" 2>/dev/null | sed 's/\.$//')
  if [[ "$cname" == "$FQDN" ]]; then
    echo "  OK  CNAME ${CUSTOM_DOMAIN} -> ${cname}"
  else
    echo "  --  CNAME ${CUSTOM_DOMAIN} -> ${FQDN:-<missing>} (got: ${cname:-none})"
    ok=0
  fi

  local txt_host="asuid.${SUBDOMAIN}.mftools.xyz"
  local txt
  txt=$(dig +short "TXT" "$txt_host" 2>/dev/null | tr -d '"')
  if [[ "$txt" == "$VERIFICATION_ID" ]]; then
    echo "  OK  TXT ${txt_host} -> ${txt}"
  else
    echo "  --  TXT ${txt_host} -> ${VERIFICATION_ID} (got: ${txt:-none})"
    ok=0
  fi

  echo
  if [[ "$ok" -eq 1 ]]; then
    echo "DNS looks good. Run: ./custom-domain.sh bind"
    return 0
  fi
  echo "DNS not ready yet."
  return 1
}

bind_domain() {
  echo "Adding hostname ${CUSTOM_DOMAIN}..."
  az containerapp hostname add \
    --name "$CONTAINERAPP_NAME" \
    --resource-group "$RESOURCE_GROUP_NAME" \
    --hostname "$CUSTOM_DOMAIN" \
    --location "$AZURE_LOCATION"

  echo "Binding managed certificate..."
  az containerapp hostname bind \
    --name "$CONTAINERAPP_NAME" \
    --resource-group "$RESOURCE_GROUP_NAME" \
    --hostname "$CUSTOM_DOMAIN" \
    --environment "$CONTAINERAPP_ENV" \
    --validation-method CNAME

  echo
  echo "Custom domain ready: https://${CUSTOM_DOMAIN}"
}

show_status() {
  az containerapp show --name "$CONTAINERAPP_NAME" --resource-group "$RESOURCE_GROUP_NAME" \
    --query "{fqdn:properties.configuration.ingress.fqdn, customDomains:properties.configuration.ingress.customDomains}" \
    -o json
}

case "${1:-dns}" in
  dns)     print_dns ;;
  check)   check_dns ;;
  bind)    check_dns && bind_domain ;;
  status)  show_status ;;
  *)       echo "Usage: $0 [dns|check|bind|status]"; exit 1 ;;
esac
