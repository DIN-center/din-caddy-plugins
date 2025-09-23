# Secret Management

Secure handling of API keys and credentials for DIN Caddy Plugins using environment variables and placeholders.

## Quick Start

### 1. Initial Setup
```bash
make secrets-init
# This creates .env.example and .env.local
# Edit .env.local with your actual API keys
```

### 2. Generate Caddyfile
```bash
make secrets
# Generates Caddyfile.generated from template with secrets
```

### 3. For CI/CD
```bash
make secrets-update-ci
# Updates GitHub Actions workflow with current secrets
```

## How It Works

1. **Template**: Caddyfile contains placeholders like `{{API_KEY_NAME}}`
2. **Secrets**: Stored in `.env.local` (local) or GitHub Secrets (CI/CD)
3. **Generation**: Script replaces placeholders with actual values
4. **Auto-discovery**: Automatically finds all placeholders - no hardcoded lists

## Adding New Secrets

1. Add placeholder in Caddyfile:
```caddyfile
https://api.example.com/{{NEW_API_KEY}}
```

2. Add to `.env.local`:
```bash
NEW_API_KEY=your-actual-key-here
```

3. Update CI/CD:
```bash
make secrets-update-ci
git add .github/workflows/deploy.yml
git commit -m "Add NEW_API_KEY"
```

4. Add to GitHub Secrets (Settings → Secrets → Actions)

## File Reference

- `Caddyfile` - Template with placeholders
- `.env.local` - Your local secrets (gitignored)
- `.env.example` - Example file with dummy values
- `scripts/generate-caddyfile.go` - Main generation script
- `.github/workflows/deploy.yml` - CI/CD workflow

## Make Commands

- `make secrets` - Generate Caddyfile from secrets
- `make secrets-init` - Initial setup
- `make secrets-update-ci` - Update GitHub workflow

## Security

- Never commit `.env.local`
- Rotate keys regularly
- Use different keys for staging/production
- All secrets are masked in preview mode