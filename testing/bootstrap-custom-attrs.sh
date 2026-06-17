#!/bin/bash
# Bootstrap script to copy custom attributes into CheckMK container
# Place in docker-entrypoint.d/pre-start/ for automatic execution

set -e

echo "[BOOTSTRAP] Copying custom attributes file..."

# Use CMK_SITE_ID from environment, default to 'cmk'
SITE_ID="${CMK_SITE_ID:-cmk}"
VOLUME_PATH="/wato"
APP_PATH="/omd/sites/${SITE_ID}/etc/check_mk/multisite.d/wato"

echo "[BOOTSTRAP] Site ID: ${SITE_ID}"
echo "[BOOTSTRAP] Target path: ${APP_PATH}"

# Create directory if it doesn't exist
mkdir -p "${APP_PATH}"

# Copy custom attributes
if [ -f "${VOLUME_PATH}/custom_attrs.mk" ]; then
    cp "${VOLUME_PATH}/custom_attrs.mk" "${APP_PATH}/custom_attrs.mk"
    chown "${SITE_ID}:${SITE_ID}" "${APP_PATH}/custom_attrs.mk"
    chmod 644 "${APP_PATH}/custom_attrs.mk"
    echo "[BOOTSTRAP] Custom attributes copied successfully"
else
    echo "[BOOTSTRAP] Warning: custom_attrs.mk not found in ${VOLUME_PATH}"
    echo "[BOOTSTRAP] Custom attributes will not be available"
fi
