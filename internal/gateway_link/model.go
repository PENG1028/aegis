// GatewayLink model — trusted gateway-to-gateway authentication.
// Each TrustedGateway represents another Aegis gateway that this gateway
// can securely forward traffic to. Traffic is authenticated via shared secret.
package gatewaylink

import "time"

// AuthType constants.
const (
	AuthSharedSecret = "shared_secret"
	AuthNone         = "none"
)

// GatewayType constants.
const (
	TypeUpstream   = "upstream"   // this gateway forwards to it
	TypeDownstream = "downstream" // receives traffic from upstream
	TypePeer       = "peer"       // mutual forwarding
)

// Status constants.
const (
	StatusActive   = "active"
	StatusDisabled = "disabled"
)

// TrustedGateway represents another gateway node that this gateway trusts.
type TrustedGateway struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Host        string    `json:"host"`                 // public IP or hostname
	PrivateIP   string    `json:"private_ip,omitempty"` // private IP for auto-routing
	Port        int       `json:"port"`                 // target port (usually 443)
	AuthType    string    `json:"auth_type"`            // shared_secret | none
	AuthValue   string    `json:"-"`                    // the shared secret (hashed in DB, never returned)
	GatewayType string    `json:"gateway_type"`         // upstream | downstream | peer
	AutoRoute   bool      `json:"auto_route"`           // prefer private IP, fallback to public
	Status      string    `json:"status"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// NewTrustedGateway creates a new trusted gateway with hashed auth.
func NewTrustedGateway(name, host, privateIP string, port int, authSecret, gatewayType string, autoRoute bool) *TrustedGateway {
	now := time.Now()
	return &TrustedGateway{
		Name:        name,
		Host:        host,
		PrivateIP:   privateIP,
		Port:        port,
		AuthType:    AuthSharedSecret,
		AuthValue:   hashSecret(authSecret),
		GatewayType: gatewayType,
		AutoRoute:   autoRoute,
		Status:      StatusActive,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
}

// ResolveHost returns the best host to connect to based on auto-routing.
// If private IP is available and auto-route is enabled, prefers private IP.
func (g *TrustedGateway) ResolveHost() string {
	if g.AutoRoute && g.PrivateIP != "" {
		return g.PrivateIP
	}
	return g.Host
}

// CheckAuth verifies a provided secret against the stored auth value.
func (g *TrustedGateway) CheckAuth(providedSecret string) bool {
	if g.AuthType != AuthSharedSecret || g.AuthValue == "" {
		return false
	}
	return g.AuthValue == hashSecret(providedSecret)
}

// RotateSecret updates the auth secret with a new hashed value.
func (g *TrustedGateway) RotateSecret(newSecret string) {
	g.AuthValue = hashSecret(newSecret)
	g.UpdatedAt = time.Now()
}
