---
title: Coverage Setup
parent: Observability
nav_order: 5
---

# Coverage Reports Setup

This project automatically generates HTML coverage reports through GitHub Actions and makes them available in multiple ways.

## 1. CI Artifacts (Always Available)

Every PR and push to main automatically generates coverage reports as downloadable artifacts:

1. Navigate to **Actions** tab in your GitHub repository
2. Click on any workflow run (e.g., "Test" workflow)
3. Scroll down to **Artifacts** section
4. Download **coverage-report** artifact
5. Extract the ZIP file and open `coverage.html` in your browser

## 2. GitHub Pages Setup (Optional)

To enable automatic deployment of coverage reports to GitHub Pages:

### Enable GitHub Pages

1. Go to **Settings** → **Pages** in your GitHub repository
2. Under **Source**, select **GitHub Actions**
3. The coverage reports will be automatically deployed to: `https://[your-org].github.io/[repo-name]/`

### Workflow Configuration

The `.github/workflows/pages.yml` workflow will:
- Run on every push to `main` branch
- Generate fresh coverage reports
- Deploy them to GitHub Pages
- Make them accessible at a public URL

### Custom Domain (Optional)

To use a custom domain:
1. Add a `CNAME` file to your repository root with your domain
2. Configure your DNS to point to GitHub Pages
3. Enable HTTPS in repository settings

## 3. Local Development

Generate coverage reports locally:

```bash
# Generate coverage report and open in browser
make coverage

# Or step by step:
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out -o coverage.html
open coverage.html  # macOS
xdg-open coverage.html  # Linux
start coverage.html  # Windows
```

## 4. Alternative: AWS S3/CloudFront

If you prefer to host coverage reports on AWS:

1. Create an S3 bucket for static hosting
2. Modify `.github/workflows/test.yml` to upload to S3:

```yaml
- name: Deploy to S3
  if: matrix.test-type == 'unit' && github.ref == 'refs/heads/main'
  run: |
    aws s3 cp coverage.html s3://your-coverage-bucket/index.html
    aws cloudfront create-invalidation --distribution-id YOUR_DISTRIBUTION_ID --paths "/*"
  env:
    AWS_ACCESS_KEY_ID: ${{ secrets.AWS_ACCESS_KEY_ID }}
    AWS_SECRET_ACCESS_KEY: ${{ secrets.AWS_SECRET_ACCESS_KEY }}
```

## Security Considerations

- **Public Repositories**: Coverage reports will be publicly accessible via GitHub Pages
- **Private Repositories**: Coverage reports are only accessible to repository collaborators
- **Sensitive Data**: Ensure no secrets or sensitive file paths are exposed in coverage reports

## Troubleshooting

### Coverage Reports Not Generating
- Check that tests are running successfully in the "unit" matrix job
- Verify the `coverage.txt` file is being created
- Ensure Go toolchain has permission to write files

### GitHub Pages Not Updating
- Check the Pages workflow run in Actions tab
- Verify Pages is enabled in repository settings
- Ensure the workflow has proper permissions (`pages: write`, `id-token: write`)

### Missing Coverage for Some Files
- Check the `-coverpkg` parameter includes your package paths
- Verify test files are properly structured and discoverable
- Review the coverage profile with `go tool cover -func=coverage.txt`
