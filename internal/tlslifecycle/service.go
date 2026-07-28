package tlslifecycle

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"time"

	"aegis/internal/certstore"
	"aegis/internal/hostdep/provider"
	"aegis/internal/route"
)

const (
	ReasonManagedByProvider = "CERT_MANAGED_BY_PROVIDER"
	ReasonHasReferences     = "CERT_HAS_REFERENCES"
)

var ErrCertificateNotFound = errors.New("certificate not found")

type RouteReference struct {
	ID     string `json:"id"`
	Domain string `json:"domain"`
}

type OperationPreview struct {
	Action       string           `json:"action"`
	Allowed      bool             `json:"allowed"`
	ReasonCode   string           `json:"reason_code,omitempty"`
	Reason       string           `json:"reason,omitempty"`
	ManagedBy    string           `json:"managed_by,omitempty"`
	References   []RouteReference `json:"references"`
	Effects      []string         `json:"effects"`
	Alternatives []string         `json:"alternatives"`
}

type BindingCandidate struct {
	RouteID              string `json:"route_id"`
	Domain               string `json:"domain"`
	CurrentCertID        string `json:"current_cert_id,omitempty"`
	CurrentBindingMode   string `json:"current_binding_mode"`
	AlreadyBound         bool   `json:"already_bound"`
	ReplacesAutomaticTLS bool   `json:"replaces_automatic_tls"`
	Selected             bool   `json:"selected"` // compatibility alias for already_bound
}

type BindingPreview struct {
	CertID     string             `json:"cert_id"`
	Domains    string             `json:"domains"`
	Candidates []BindingCandidate `json:"candidates"`
}

type RouteDeletePreview struct {
	Action                         string   `json:"action"`
	Allowed                        bool     `json:"allowed"`
	RouteID                        string   `json:"route_id"`
	Domain                         string   `json:"domain"`
	TLSBindingMode                 string   `json:"tls_binding_mode"`
	CertificateID                  string   `json:"certificate_id,omitempty"`
	DeleteUnusedCertificateAllowed bool     `json:"delete_unused_certificate_allowed"`
	Effects                        []string `json:"effects"`
}

type Service struct {
	routes    *route.AppService
	certs     *certstore.Service
	providers *provider.Registry
}

func New(routes *route.AppService, certs *certstore.Service, providers *provider.Registry) *Service {
	return &Service{routes: routes, certs: certs, providers: providers}
}

func (s *Service) PreviewDeleteCertificate(ctx context.Context, certID string) (*OperationPreview, error) {
	cert, err := s.certs.Get(certID)
	if err != nil {
		return nil, err
	}
	if cert == nil {
		return nil, fmt.Errorf("%w: %s", ErrCertificateNotFound, certID)
	}
	preview := &OperationPreview{
		Action:       "delete_certificate",
		Allowed:      true,
		ManagedBy:    managedBy(cert.Source),
		References:   []RouteReference{},
		Effects:      []string{"delete certificate metadata and Aegis-managed PEM files"},
		Alternatives: []string{},
	}
	if cert.Source == certstore.SourceGatewayAuto {
		preview.Allowed = false
		preview.ReasonCode = ReasonManagedByProvider
		preview.Reason = "certificate lifecycle is managed by its automatic TLS executor; change the route TLS strategy instead"
		preview.Effects = []string{}
		preview.Alternatives = []string{"change_tls_strategy", "retire_exposure"}
		return preview, nil
	}

	refs, err := s.routes.FindRoutesByCertID(ctx, certID)
	if err != nil {
		return nil, err
	}
	for _, ref := range refs {
		preview.References = append(preview.References, RouteReference{ID: ref.ID, Domain: ref.Domain})
	}
	if len(preview.References) > 0 {
		preview.Allowed = false
		preview.ReasonCode = ReasonHasReferences
		preview.Reason = "certificate is still bound to routes; replace or unbind it first"
		preview.Effects = []string{}
		preview.Alternatives = []string{"replace_tls_binding", "unbind_certificate"}
	}
	return preview, nil
}

func (s *Service) DeleteCertificate(ctx context.Context, certID string) (*OperationPreview, error) {
	preview, err := s.PreviewDeleteCertificate(ctx, certID)
	if err != nil || !preview.Allowed {
		return preview, err
	}
	if err := s.certs.Delete(certID); err != nil {
		if errors.Is(err, certstore.ErrCertificateReferenced) {
			// A route may have been bound after the preview. Re-read so callers get
			// the same structured 409 response instead of a storage error.
			return s.PreviewDeleteCertificate(ctx, certID)
		}
		return nil, err
	}
	return preview, nil
}

func (s *Service) BindCertificate(ctx context.Context, routeID, certID string) (*route.Route, error) {
	cert, err := s.certs.Get(certID)
	if err != nil {
		return nil, err
	}
	if cert == nil {
		return nil, fmt.Errorf("certificate %s not found", certID)
	}
	if cert.Source == certstore.SourceGatewayAuto {
		return nil, fmt.Errorf("provider-managed certificates cannot be bound as PEM assets")
	}
	if !certstore.ValidAt(cert, time.Now()) {
		return nil, fmt.Errorf("certificate is not currently valid")
	}
	rt, err := s.routes.GetRoute(ctx, routeID)
	if err != nil {
		return nil, err
	}
	if s.providers != nil && s.providerForCapability(rt.SourceProvider, provider.CapLoadCert) == "" {
		return nil, fmt.Errorf("active runtime mode cannot load certificate assets")
	}
	if !certstore.CoversDomain(cert, rt.Domain) {
		return nil, fmt.Errorf("certificate does not cover domain %s", rt.Domain)
	}
	return s.routes.SetTLSBinding(ctx, routeID, route.TLSBindingCertificate, "", certID)
}

func (s *Service) UseProviderAuto(ctx context.Context, routeID, providerID string) (*route.Route, error) {
	rt, err := s.routes.GetRoute(ctx, routeID)
	if err != nil {
		return nil, err
	}
	// WHY: provider-managed TLS state is executor-sticky. This ordinary binding
	// operation may select an executor only for a new binding; moving an existing
	// binding belongs to the explicit mode-migration workflow.
	if rt.TLSBindingMode == route.TLSBindingProviderAuto && rt.TLSProvider != "" {
		if providerID != "" && providerID != rt.TLSProvider {
			return nil, fmt.Errorf("automatic TLS is bound to executor %s; explicit migration is required", rt.TLSProvider)
		}
		providerID = rt.TLSProvider
		if s.providers != nil && !s.providerSupportsCapability(providerID, provider.CapAutoCert) {
			return nil, fmt.Errorf("automatic TLS execution binding %s is unavailable; explicit migration is required", providerID)
		}
		return s.routes.SetTLSBinding(ctx, routeID, route.TLSBindingProviderAuto, providerID, "")
	}
	if providerID == "" {
		if s.providers == nil {
			return nil, fmt.Errorf("automatic TLS provider is unavailable")
		}
		providerID = s.providerForCapability(rt.SourceProvider, provider.CapAutoCert)
		if providerID == "" {
			return nil, fmt.Errorf("no provider supports automatic certificates")
		}
	}
	if s.providers != nil && !s.providerSupportsCapability(providerID, provider.CapAutoCert) {
		return nil, fmt.Errorf("selected executor cannot provide automatic TLS in the active runtime mode")
	}
	return s.routes.SetTLSBinding(ctx, routeID, route.TLSBindingProviderAuto, providerID, "")
}

func (s *Service) providerSupportsCapability(providerID string, capability provider.Capability) bool {
	if s.providers == nil || providerID == "" {
		return false
	}
	mode := provider.DetectRuntimeMode(s.providers.List())
	if !slices.Contains(mode.ProviderIDs(), providerID) {
		return false
	}
	p := s.providers.Get(providerID)
	return p != nil && p.State().HasCapability(capability)
}

func (s *Service) providerForCapability(preferred string, capability provider.Capability) string {
	if s.providers == nil {
		return ""
	}
	mode := provider.DetectRuntimeMode(s.providers.List())
	if preferred != "" && slices.Contains(mode.ProviderIDs(), preferred) {
		if p := s.providers.Get(preferred); p != nil && p.State().HasCapability(capability) {
			return preferred
		}
	}
	for _, providerID := range mode.ProviderIDs() {
		if p := s.providers.Get(providerID); p != nil && p.State().HasCapability(capability) {
			return providerID
		}
	}
	return ""
}

// PreviewCertificateBindings returns only TLS-terminating routes covered by the
// certificate. It never changes bindings; wildcard matching follows RFC 6125's
// single-label rule in certstore.CoversDomain.
func (s *Service) PreviewCertificateBindings(ctx context.Context, certID string) (*BindingPreview, error) {
	cert, err := s.certs.Get(certID)
	if err != nil {
		return nil, err
	}
	if cert == nil {
		return nil, fmt.Errorf("%w: %s", ErrCertificateNotFound, certID)
	}
	if cert.Source == certstore.SourceGatewayAuto {
		return nil, fmt.Errorf("provider-managed automatic TLS state is not a bindable certificate asset")
	}
	if !certstore.ValidAt(cert, time.Now()) {
		return nil, fmt.Errorf("certificate is not currently valid")
	}
	routes, err := s.routes.ListRoutes(ctx)
	if err != nil {
		return nil, err
	}
	preview := &BindingPreview{CertID: cert.ID, Domains: cert.Domains, Candidates: []BindingCandidate{}}
	for _, rt := range routes {
		def := rt.CompDef()
		if def == nil || def.TLSMode != "terminate" || !certstore.CoversDomain(cert, rt.Domain) {
			continue
		}
		currentCertID := ""
		if rt.CertID != nil {
			currentCertID = *rt.CertID
		}
		alreadyBound := currentCertID == certID
		preview.Candidates = append(preview.Candidates, BindingCandidate{
			RouteID: rt.ID, Domain: rt.Domain, CurrentCertID: currentCertID,
			CurrentBindingMode: rt.TLSBindingMode,
			AlreadyBound:       alreadyBound, Selected: alreadyBound,
			ReplacesAutomaticTLS: rt.TLSBindingMode == route.TLSBindingProviderAuto,
		})
	}
	return preview, nil
}

// BindCertificateToRoutes validates the complete selection before committing
// one atomic database update.
func (s *Service) BindCertificateToRoutes(ctx context.Context, certID string, routeIDs []string) (*BindingPreview, error) {
	preview, err := s.PreviewCertificateBindings(ctx, certID)
	if err != nil {
		return nil, err
	}
	allowed := make(map[string]BindingCandidate, len(preview.Candidates))
	for _, candidate := range preview.Candidates {
		allowed[candidate.RouteID] = candidate
	}
	unique := make([]string, 0, len(routeIDs))
	seen := make(map[string]bool, len(routeIDs))
	for _, routeID := range routeIDs {
		if seen[routeID] {
			continue
		}
		if _, ok := allowed[routeID]; !ok {
			return nil, fmt.Errorf("route %s is not an eligible certificate binding", routeID)
		}
		rt, err := s.routes.GetRoute(ctx, routeID)
		if err != nil {
			return nil, err
		}
		if s.providers != nil && s.providerForCapability(rt.SourceProvider, provider.CapLoadCert) == "" {
			return nil, fmt.Errorf("active runtime mode cannot load certificate assets for route %s", routeID)
		}
		seen[routeID] = true
		unique = append(unique, routeID)
	}
	if len(unique) == 0 {
		return nil, fmt.Errorf("at least one eligible route is required")
	}
	if err := s.routes.SetCertificateBindings(ctx, unique, certID); err != nil {
		return nil, err
	}
	return s.PreviewCertificateBindings(ctx, certID)
}

func (s *Service) PreviewDeleteRoute(ctx context.Context, routeID string) (*RouteDeletePreview, error) {
	rt, err := s.routes.GetRoute(ctx, routeID)
	if err != nil {
		return nil, err
	}
	preview := &RouteDeletePreview{
		Action: "delete_route", Allowed: true, RouteID: rt.ID, Domain: rt.Domain,
		TLSBindingMode: rt.TLSBindingMode,
		Effects:        []string{"delete the route and its TLS binding", "retain independent certificate assets by default"},
	}
	if rt.CertID == nil || *rt.CertID == "" {
		return preview, nil
	}
	preview.CertificateID = *rt.CertID
	cert, err := s.certs.Get(*rt.CertID)
	if err != nil || cert == nil || cert.Source == certstore.SourceGatewayAuto {
		return preview, err
	}
	refs, err := s.routes.FindRoutesByCertID(ctx, cert.ID)
	if err != nil {
		return nil, err
	}
	preview.DeleteUnusedCertificateAllowed = len(refs) == 1 && refs[0].ID == rt.ID
	return preview, nil
}

func managedBy(source string) string {
	switch source {
	case certstore.SourceLocalACME:
		return "aegis"
	case certstore.SourceManualUpload, certstore.SourceExternal:
		return "user"
	case certstore.SourceGatewayAuto:
		return "executor"
	default:
		return "unknown"
	}
}
