#!/bin/bash
# Fix Caddyfile for Aegis panel with TLS

cat > /etc/caddy/Caddyfile << 'EOF'
# Aegis Control Panel
# TLS: automatic Let's Encrypt

aegis.nexorastack.com {
    handle /api/* {
        reverse_proxy 127.0.0.1:7380 {
            header_up Host {host}
            header_up X-Forwarded-For {remote_host}
            header_up X-Forwarded-Proto {scheme}
        }
    }
    handle {
        reverse_proxy 127.0.0.1:7380 {
            header_up Host {host}
            header_up X-Forwarded-For {remote_host}
            header_up X-Forwarded-Proto {scheme}
        }
    }
}

:80 {
    redir https://{host}{uri} permanent
}

e2e-test.depotly.internal {
    encode gzip
    reverse_proxy http://127.0.0.1:5432
}

pg-test.depotly.internal {
    encode gzip
    reverse_proxy http://127.0.0.1:5432
}

runsping.vps2.internal {
    encode gzip
    reverse_proxy http://$${SERVER_A:?set SERVER_A}:80 {
        header_up X-Aegis-Gateway-Link "gw_ebaae1982975be7b"
        header_up Host "runsping.vps2.internal"
        header_up X-Aegis-Gateway-Token "9e10e1285081d22723013b4db0f1c870ed6258037107614c04b6cbd7b731b0da"
    }
}
EOF

chown root:caddy /etc/caddy/Caddyfile
chmod 640 /etc/caddy/Caddyfile
caddy validate --config /etc/caddy/Caddyfile --adapter caddyfile
systemctl start caddy
systemctl is-active caddy
