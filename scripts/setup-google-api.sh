#!/bin/bash

# Google API Setup Script

echo "Setting up Google API credentials..."
echo ""
echo "Please follow these steps to set up Google API access:"
echo ""
echo "1. Go to the Google Cloud Console: https://console.cloud.google.com/"
echo "2. Create a new project or select an existing one"
echo "3. Enable the following APIs:"
echo "   - Google Drive API"
echo "   - Google Sheets API"
echo ""
echo "4. Create credentials:"
echo "   - Go to 'Credentials' in the left sidebar"
echo "   - Click 'Create Credentials' > 'OAuth 2.0 Client IDs'"
echo "   - Choose 'Web application'"
echo "   - Add authorized redirect URI: http://localhost:8030/api/auth/callback"
echo ""
echo "5. Download the credentials JSON file"
echo "6. Copy the Client ID and Client Secret to the application configuration"
echo ""
echo "7. Create a Google Sheet for logging:"
echo "   - Go to https://sheets.google.com/"
echo "   - Create a new spreadsheet"
echo "   - Name the first sheet 'Logs'"
echo "   - Copy the spreadsheet ID from the URL"
echo "   - Add the ID to the application configuration"
echo ""
echo "8. Share the Google Sheet with your Google account"
echo "9. Create a Google Drive folder for backups (optional)"
echo "   - Copy the folder ID from the URL"
echo "   - Add the ID to the application configuration"
echo ""
echo "Setup complete! You can now configure the application through the web interface."
