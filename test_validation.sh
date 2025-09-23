#!/bin/bash
echo "Preparing Caddyfile for validation..."

# Create a copy for validation
cp Caddyfile Caddyfile.validate

# Create a dummy 256-bit hex secret key for validation
echo "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef" > secret.key

# Replace the secret path with local path
sed -i '' 's|/run/secrets/din-secret-key|./secret.key|g' Caddyfile.validate

# Fix log format syntax - format must be outside the file block
# First, extract the format line if it exists inside the file block
sed -i '' '/output file caddy.log {/,/^[[:space:]]*}/ {
  /format json/d
}' Caddyfile.validate

# Now add format at the correct level (directly under log)
sed -i '' '/^[[:space:]]*log {/a\
\		format json
' Caddyfile.validate

# Show the fixed log section for verification
echo "Fixed log section:"
sed -n '/^[[:space:]]*log {/,/^[[:space:]]*}/p' Caddyfile.validate | head -10

# Validate the Caddyfile
echo "Validating Caddyfile..."
./caddy-din validate --config Caddyfile.validate --adapter caddyfile

# Clean up validation files
rm -f Caddyfile.validate secret.key

echo "✅ Caddyfile validation successful!"
