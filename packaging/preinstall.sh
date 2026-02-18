#!/bin/sh
getent group ns116 >/dev/null || groupadd -r ns116
getent passwd ns116 >/dev/null || useradd -r -g ns116 -M -s /sbin/nologin -c "NS116 DNS Manager" ns116
