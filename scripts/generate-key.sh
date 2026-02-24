#!/bin/bash

# Generates a random 32-character hex string suitable for AES-256 encryption.
# Uses OpenSSL to read 16 cryptographically secure random bytes and output as 32 hex chars.

# Generate 16 bytes of secure random data and format it as 32 hex characters
KEY=$(openssl rand -hex 16)
echo "Generated ENCRYPTION_KEY (32 hex characters = exactly 32 bytes string):"
echo "────────────────────────────────"
echo "$KEY"
echo "────────────────────────────────"
echo ""

# Write to .env file if it doesn't already contain ENCRYPTION_KEY
ENV_FILE=".env"
if [ ! -f "$ENV_FILE" ]; then
    echo "ENCRYPTION_KEY=$KEY" > "$ENV_FILE"
    echo "Created new $ENV_FILE and appended ENCRYPTION_KEY."
elif ! grep -q "^ENCRYPTION_KEY=" "$ENV_FILE"; then
    echo "ENCRYPTION_KEY=$KEY" >> "$ENV_FILE"
    echo "Appended ENCRYPTION_KEY to existing $ENV_FILE."
else
    echo "⚠️  ENCRYPTION_KEY is already set in your $ENV_FILE!"
    echo "   If you replace your current key, ALL existing saved credentials"
    echo "   in the Bot Portal will become unreadable and must be recreated."
    echo ""
    echo "   To replace it manually, update this line in your $ENV_FILE:"
    echo "   ENCRYPTION_KEY=$KEY"
fi

echo ""
echo "Done!"
