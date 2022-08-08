#!/bin/bash

echo "Moving binary..."
mv ./nocloud-ione-vnc /usr/bin/
echo "Moving unit file..."
mv ./nocloud-ione-vnc.service /usr/lib/systemd/system

echo "Reloading daemons..."
systemctl daemon-reload
echo "Enabling daemon..."
systemctl enable nocloud-ione-vnc
echo "Starting daemon..."
systemctl start nocloud-ione-vnc

echo "Don't forget to setup Nginx"