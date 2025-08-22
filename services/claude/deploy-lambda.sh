#!/bin/bash

# Simple Lambda deployment script
echo "🚀 Preparing Lambda deployment package..."

# Clean and rebuild
echo "Building TypeScript..."
npm run build

# Create deployment directory
rm -rf lambda-deploy
mkdir lambda-deploy

# Copy necessary files
echo "Copying files..."
cp -r dist lambda-deploy/
cp -r node_modules lambda-deploy/
cp lambda-handler.js lambda-deploy/
cp package.json lambda-deploy/

# Create the zip file
echo "Creating zip package..."
cd lambda-deploy
zip -r ../lambda-function.zip . -q
cd ..

# Clean up
rm -rf lambda-deploy

echo "✅ Lambda package ready: lambda-function.zip"
echo ""
echo "📦 Package size: $(du -h lambda-function.zip | cut -f1)"
echo ""
echo "Next steps:"
echo "1. Go to AWS Lambda Console"
echo "2. Create new function (Node.js 18.x runtime)"
echo "3. Upload lambda-function.zip"
echo "4. Set environment variables:"
echo "   - SIGNOZ_API_KEY"
echo "   - CLAUDE_API_KEY"
echo "   - SIGNOZ_BASE_URL (optional)"
echo "5. Set handler to: lambda-handler.handler"
echo "6. Set timeout to: 30 seconds"
echo "7. Set memory to: 512 MB"
echo "8. Create Function URL (no auth for testing)"