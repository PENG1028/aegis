package routingtable

// Service provides routing table generation and preview capabilities.
type Service struct {
	gen *Generator
}

// NewService creates a new routing table service.
func NewService() *Service {
	return &Service{gen: NewGenerator()}
}

// Generate produces a routing table for the given input.
func (s *Service) Generate(input GenerateInput) (*RoutingTable, error) {
	return s.gen.Generate(input)
}

// Preview generates a routing table without persisting it.
// Returns the routing table and any validation results.
func (s *Service) Preview(input GenerateInput) (*RoutingTable, *ValidationResult, error) {
	table, err := s.gen.Generate(input)
	if err != nil {
		return nil, nil, err
	}
	validation := Validate(table)
	return table, validation, nil
}

// ValidateTable validates a generated routing table.
func (s *Service) ValidateTable(table *RoutingTable) *ValidationResult {
	return Validate(table)
}
