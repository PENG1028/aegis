export const mockSettings: Record<string, any> = {
  admin: {
    username: 'admin',
    session_timeout: '8h',
    auth_mode: 'password',
  },
  node_identity: {
    current_node_id: 'node-a',
    private_ip: '10.0.1.4',
    public_ip: '<SERVER_A_IP>',
  },
  gateway_defaults: {
    default_listener: '0.0.0.0:80',
    default_provider: 'caddy_http',
    gateway_mode: 'edge_mux + caddy',
  },
  relay_defaults: {
    default_mode: 'public_gateway',
    max_hop: 1,
    target_suppressed: true,
  },
  safety_defaults: {
    warn_mode: 'log only',
    block_mode: 'disabled',
    auto_detect_public_target: true,
  },
  secret_key: {
    key_path: '/etc/aegis/secret.key',
    key_format: 'PEM',
    key_rotation: 'manual',
  },
};
