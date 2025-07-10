#!/bin/bash

# MySQL Backup System Installation Script

set -e

echo "Installing MySQL Backup System..."

# Check if Go is installed
if ! command -v go &> /dev/null; then
    echo "Go is not installed. Please install Go 1.21 or later."
    exit 1
fi

# Check if MySQL client is installed
if ! command -v mysqldump &> /dev/null; then
    echo "MySQL client (mysqldump) is not installed. Please install MySQL client."
    exit 1
fi

# Create application directory
APP_DIR="/opt/mysql-backup-system"
sudo mkdir -p $APP_DIR

# Copy application files
sudo cp -r . $APP_DIR/
cd $APP_DIR

# Build the application
echo "Building application..."
sudo go mod tidy
sudo go build -o mysql-backup-system main.go

# Make executable
sudo chmod +x mysql-backup-system

# Create systemd service file
echo "Creating systemd service..."
sudo tee /etc/systemd/system/mysql-backup-system.service > /dev/null <<EOF
[Unit]
Description=MySQL Backup System
After=network.target mysql.service

[Service]
Type=simple
User=root
WorkingDirectory=$APP_DIR
ExecStart=$APP_DIR/mysql-backup-system -service
Restart=always
RestartSec=10

[Install]
WantedBy=multi-user.target
EOF

# Create web service file
sudo tee /etc/systemd/system/mysql-backup-web.service > /dev/null <<EOF
[Unit]
Description=MySQL Backup System Web Interface
After=network.target

[Service]
Type=simple
User=root
WorkingDirectory=$APP_DIR
ExecStart=$APP_DIR/mysql-backup-system -port=8030
Restart=always
RestartSec=10

[Install]
WantedBy=multi-user.target
EOF

# Reload systemd and enable services
sudo systemctl daemon-reload
sudo systemctl enable mysql-backup-web.service

echo "Installation completed!"
echo ""
echo "To start the web interface:"
echo "  sudo systemctl start mysql-backup-web"
echo ""
echo "To enable automatic backups:"
echo "  sudo systemctl enable mysql-backup-system"
echo "  sudo systemctl start mysql-backup-system"
echo ""
echo "Access the web interface at: http://localhost:8030"
echo ""
echo "Configuration file will be created at: ~/.mysql-backup-config"
