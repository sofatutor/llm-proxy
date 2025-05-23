# Admin UI Testing Guide

## Prerequisites
1. Make sure you have a `.env` file with `MANAGEMENT_TOKEN` set
2. Build the project: `go build -o tmp/llm-proxy ./cmd/proxy`

## Testing Steps

### 1. Start the main server (in terminal 1):
```bash
./tmp/llm-proxy server --log-level debug
```

### 2. Start the admin UI (in terminal 2):
```bash
./tmp/llm-proxy admin
```

### 3. Open browser and test:
1. Go to http://localhost:8081
2. You should see the login screen
3. Enter your management token: `4cb90213b1c1f48f675eab059abb0774a10d82e8a553a7a83fbf5ad47c896116`
4. You should be redirected to the dashboard

### 4. Test the list views:
1. **Projects List**: Click "Projects" in navigation
   - Should show projects with obfuscated API keys
   - API keys should show like: `sk-1234...abcd`
   
2. **Tokens List**: Click "Tokens" in navigation  
   - Should show tokens without actual token values (sanitized)
   - Should show status, expiration, request counts
   - Should handle null pointers for ExpiresAt and MaxRequests

3. **Project Details**: Click on a project
   - Should show full project details
   - API key should be in password field (hidden by default)
   - Should have toggle button to show/hide API key

### 5. Test logout:
1. Click the "Logout" button in navigation
2. Should redirect to login screen
3. Should clear session (try going back to dashboard URL directly)

## Expected Behavior

✅ **Authentication**:
- Only valid management tokens can log in
- Invalid tokens show error message
- Sessions persist (with remember me for 30 days)
- Logout clears session and redirects

✅ **Security**:
- API keys are obfuscated in list views (`sk-1234...abcd`)
- Token values are never shown (sanitized by API)
- Sessions use secure cookies

✅ **List Views**:
- Projects show with obfuscated API keys
- Tokens show sanitized data (no actual token values)
- Proper handling of null/pointer fields
- Responsive design works on mobile

✅ **Navigation**:
- All pages have logout button
- Breadcrumbs and navigation work
- Error handling for invalid requests