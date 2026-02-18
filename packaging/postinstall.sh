#!/bin/sh
systemctl daemon-reload
echo ""
echo "  NS116 installed successfully."
echo ""
echo "  1. Edit /etc/ns116.yaml with your AWS credentials"
echo "  2. systemctl enable --now ns116"
echo ""
